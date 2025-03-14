package scrambler

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPartition_Examples_ProduceExpectedPartition(t *testing.T) {
	tests := map[string]struct {
		transactions []transaction
		partitions   [][]int
	}{
		"empty": {
			[]transaction{},
			[][]int{},
		},
		"single": {
			[]transaction{
				tx(a(0, 0)),
			},
			[][]int{{0}},
		},
		"independent": {
			[]transaction{
				tx(a(0, 0)),
				tx(a(1, 0)),
			},
			[][]int{{0}, {1}},
		},
		"dependent": {
			[]transaction{
				tx(a(0, 0)),
				tx(a(0, 1)),
			},
			[][]int{{0, 1}},
		},
		"chain": {
			[]transaction{
				tx(a(0, 0), a(1, 0)),
				tx(a(1, 1), a(2, 0)),
				tx(a(2, 1), a(3, 0)),
				tx(a(3, 1), a(4, 0)),
			},
			[][]int{{0, 1, 2, 3}},
		},
		"two chains": {
			[]transaction{
				tx(a(0, 0), a(1, 0)),
				tx(a(7, 1), a(8, 0)),
				tx(a(1, 1), a(2, 0)),
				tx(a(8, 1), a(9, 0)),
			},
			[][]int{{0, 2}, {1, 3}},
		},
	}

	impls := []func([]transaction) [][]transaction{
		partition,
		partition1,
		partition2,
	}
	for i, impl := range impls {
		t.Run(fmt.Sprintf("impl-%d", i), func(t *testing.T) {
			for name, tt := range tests {
				t.Run(name, func(t *testing.T) {
					require := require.New(t)
					got := impl(tt.transactions)
					//require.Equal(len(tt.partitions), len(got))

					// convert the partition into indices to make comparison easier
					res := [][]int{}
					for _, transactions := range got {
						res = append(res, toIndices(t, tt.transactions, transactions))
					}

					// the result can be in an arbitrary order, so we need to sort the
					// partitions before comparing
					for i := range res {
						slices.Sort(res[i])
					}
					sort.Slice(res, func(i, j int) bool {
						return less(res[i], res[j])
					})
					require.Equal(tt.partitions, res)
				})
			}
		})
	}
}

func toIndices(
	t *testing.T,
	full []transaction,
	sub []transaction,
) []int {
	t.Helper()
	var indices []int
	for _, cur := range sub {
		pos := slices.IndexFunc(full, func(tx transaction) bool {
			return reflect.DeepEqual(tx, cur)
		})
		require.NotEqual(t, -1, pos)
		indices = append(indices, pos)
	}
	return indices
}

func TestSortPartition_KnownExamples_ProduceExpectedResult(t *testing.T) {

	tests := map[string]struct {
		transactions []transaction
		result       []int
	}{
		"empty": {},
		"only one transaction": {
			[]transaction{
				tx(a(0, 0)),
			},
			[]int{0},
		},
		"mutual exclusion": {
			[]transaction{
				tx(a(10, 2), a(12, 1)),
				tx(a(12, 2), a(10, 1)),
			},
			nil, // neither of the transactions can be processed first
		},
		"mutual reference without preference": {
			[]transaction{
				tx(a(10, 1), a(12, 2)),
				tx(a(12, 1), a(10, 2)),
			},
			[]int{0, 1}, // this could also be {1, 0}, but the implementation is deterministic
		},
		"mutual reference with preference": {
			[]transaction{
				tx(a(10, 1), a(12, 2)),
				tx(a(12, 1), a(10, 2)),
				tx(a(12, 3)),
			},
			[]int{1, 0, 2}, // this is unique, since starting with 0 would prevent 2 from being processed
		},
		"scrambled chain": {
			[]transaction{
				tx(a(0, 2)),
				tx(a(0, 0)),
				tx(a(0, 1)),
				tx(a(0, 3)),
				tx(a(0, 4)),
			},
			[]int{1, 2, 0, 3, 4},
		},
		"authentication enabling a transaction": {
			[]transaction{
				tx(a(10, 2), a(12, 1)),
				tx(a(12, 2), a(10, 1)),
				tx(a(8, 1), a(10, 1)),
			},
			[]int{2, 0, 1},
		},
		"authentication enabling follow-up": {
			[]transaction{
				tx(a(10, 3)),
				tx(a(10, 5)),
				tx(a(12, 1), a(10, 4)),
			},
			[]int{0, 2, 1},
		},
		"self authorization": {
			[]transaction{
				tx(a(10, 1), a(10, 2), a(10, 3)),
				tx(a(10, 4)),
			},
			[]int{0, 1},
		},
		"invalid self authorization is ignored": {
			[]transaction{
				tx(a(10, 1), a(10, 3), a(10, 2)),
				tx(a(10, 4)),
			},
			[]int{0},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			// make sure the test examples contain connected transactions
			parts := partition(tt.transactions)
			require.LessOrEqual(len(parts), 1)

			got := sortPartition(tt.transactions, pickOptimal)
			res := toIndices(t, tt.transactions, got)
			require.Equal(tt.result, res)
		})
	}
}

func TestInterleaving_Examples_ProduceExpectedInterleaving(t *testing.T) {
	test := map[string]struct {
		partition [][]int
		result    []int
	}{
		"empty": {},
		"single partition with one element": {
			[][]int{{0}},
			[]int{0},
		},
		"single partition with multiple elements": {
			[][]int{{0, 1, 2}},
			[]int{0, 1, 2},
		},
		"two partitions with one element each": {
			[][]int{{0}, {1}},
			[]int{0, 1},
		},
		"two partitions with multiple elements each": {
			[][]int{{0, 1, 2}, {3, 4, 5}},
			[]int{0, 3, 1, 4, 2, 5},
		},
		"two partitions with different lengths": {
			[][]int{{0, 1, 2}, {3}},
			[]int{0, 3, 1, 2},
		},
		"three partitions with different lengths": {
			[][]int{{0, 1, 2}, {3}, {4, 5}},
			[]int{0, 3, 4, 1, 5, 2},
		},
	}

	for name, tt := range test {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			got := interleavePartitions(tt.partition)
			require.Equal(tt.result, got)
		})
	}
}

func a(s, n int) action {
	return action{s, n}
}

func tx(m action, a ...action) transaction {
	return transaction{m, a}
}

func less(a, b []int) bool {
	for i := range a {
		if i >= len(b) {
			return false
		}
		if a[i] < b[i] {
			return true
		}
		if a[i] > b[i] {
			return false
		}
	}
	return len(a) != len(b)
}

func BenchmarkPartitioning_Chains(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			transactions := make([]transaction, n)
			transactions[0] = tx(a(0, 0))
			for i := 1; i < n; i++ {
				transactions[i] = tx(a(i-1, 1), a(i, 0))
			}
			b.ResetTimer()
			for range b.N {
				partition := partition(transactions)
				if len(partition) != 1 {
					b.Fatal("unexpected partition count")
				}
			}
		})
	}
}

func BenchmarkPartitioning_ExtensiveAuthorizations(b *testing.B) {
	for _, n := range []int{1, 10, 100, 1_000, 10_000, 100_000} {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			transactions := make([]transaction, 1)
			authorizations := make([]action, n)
			for i := 0; i < n; i++ {
				authorizations[i] = a(i, 0)
			}
			transactions[0] = tx(a(0, 0), authorizations...)
			b.ResetTimer()
			for range b.N {
				partition := partition(transactions)
				if len(partition) != 1 {
					b.Fatal("unexpected partition count")
				}
			}
		})
	}
}

func FuzzGetExecutionOrder_ProducesAFullyExecutableTransactionOrder(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		sorted := GetExecutionOrder(transactions)

		// Compute the initial state of nonces indicated by the transactions.
		state := state{}
		for _, tx := range transactions {
			for _, a := range tx.actions() {
				if nonce, found := state[a.sender]; !found || a.nonce < nonce {
					state[a.sender] = a.nonce
				}
			}
		}

		// Check that all transactions can be executed in the given order.
		for _, tx := range sorted {
			if state[tx.main.sender] != tx.main.nonce {
				t.Fatalf("unable to execute transaction %v in %v on state %v", tx, sorted, state)
			}
			state.apply(tx)
		}
	})
}

func FuzzGetExecutionOrder_FindDiffBetweenOptimalAndPickFirst(f *testing.F) {
	// This fuzzer test helps to identify cases where the optimal and the
	// first-pick strategy produce different results. This can help to identify
	// edge cases where the heuristic needs to be improved.
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		a := GetExecutionOrder(transactions, pickFirst)
		b := GetExecutionOrder(transactions, pickOptimal)

		if len(a) != len(b) {
			t.Fatalf("different lengths: %v vs %v", a, b)
		}
	})
}

// parseTransactions decodes the given list of bytes into a list of transactions.
// The format is as follows:
//   - Each transaction starts with a sender and a nonce. The sender is a
//     single byte. The nonce is a single byte.
//   - The transaction is followed by a list of authorizations. Each
//     authorization consists of a sender and a nonce.
//   - The list of authorizations is terminated by a special sender 0xff.
//   - The transaction list ends if there is no more data.
func parseTransactions(data []byte) []transaction {
	var res []transaction
	for {
		var cur transaction
		if len(data) == 0 {
			return res
		}
		sender := data[0]
		data = data[1:]

		if len(data) == 0 {
			return res
		}
		nonce := data[0]
		data = data[1:]

		cur.main = action{int(sender), int(nonce)}

		for {
			if len(data) == 0 {
				res = append(res, cur)
				return res
			}
			sender = data[0]
			data = data[1:]

			if sender == 0xff {
				break
			}

			if len(data) == 0 {
				res = append(res, cur)
				return res
			}
			nonce = data[0]
			data = data[1:]

			cur.auth = append(cur.auth, action{int(sender), int(nonce)})
		}

		res = append(res, cur)
	}
}
