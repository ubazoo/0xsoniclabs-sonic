package scrambler

import (
	"fmt"
	"iter"
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
		"maximizes authorizations": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(10, 1), a(12, 1)),
			},
			[]int{1},
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

func TestSortPartition_HighestPotential_KeyExamples(t *testing.T) {

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
		"prefers transactions with more authorizations": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(10, 1), a(12, 1)), // TODO: should respect gas prices for replacements
			},
			[]int{1},
		},
		"identifies authorizations as an enabler": {
			[]transaction{
				tx(a(10, 2)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{1, 0},
		},
		"identifies authorizations as a disabler": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"identifies authorizations as a disabler (2)": { // TODO: make all tests order-invariant
			[]transaction{
				tx(a(12, 1), a(10, 1)), // < this authorization should have negative potential
				tx(a(10, 1)),
			},
			[]int{1, 0},
		},
		"identify collision with transaction action": {
			[]transaction{
				tx(a(10, 1), a(10, 1)), // < this authorization should have neutral potential
				tx(a(12, 1), a(14, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		/* This one is not supported ...
		"delays authentications if another transaction could enable it": {
			[]transaction{
				tx(a(10, 1), a(12, 2)),
				tx(a(12, 1)),
			},
			[]int{1, 0},
		},
		*/
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			// make sure the test examples contain connected transactions
			parts := partition(tt.transactions)
			require.LessOrEqual(len(parts), 1)

			got := sortPartition(tt.transactions, pickHighestPotential)
			res := toIndices(t, tt.transactions, got)
			require.Equal(tt.result, res)
		})
	}
}

func TestSortPartition2_KeyExamples(t *testing.T) {

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
		"prefers transactions with more authorizations": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(10, 1), a(12, 1)), // TODO: should respect gas prices for replacements
			},
			[]int{1},
		},
		"identifies authorizations as an enabler": {
			[]transaction{
				tx(a(10, 2)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{1, 0},
		},
		"identifies authorizations as a disabler": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"identifies authorizations as a disabler (2)": { // TODO: make all tests order-invariant
			[]transaction{
				tx(a(12, 1), a(10, 1)), // < this authorization should have negative potential
				tx(a(10, 1)),
			},
			[]int{1, 0},
		},
		"identify collision with transaction action": {
			[]transaction{
				tx(a(10, 1), a(10, 1)),
				tx(a(12, 1), a(14, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"delays authentications if another transaction could enable it": {
			[]transaction{
				tx(a(10, 1), a(12, 2)),
				tx(a(12, 1)),
			},
			[]int{1, 0},
		},
		"favor longer authentication lists": {
			[]transaction{
				tx(a(10, 1), a(10, 1), a(12, 1)),
				tx(a(12, 1), a(10, 1), a(10, 1), a(14, 2)),
			},
			[]int{1},
		},
		"favor self-authorized transaction over regular transaction": {
			[]transaction{
				tx(a(10, 1), a(10, 2)),
				tx(a(10, 1)),
			},
			[]int{0},
		},
		"authentication should not be processed if it blocks a transaction": {
			[]transaction{
				// both are ready, neither has all authorizations ready to be
				// processed, but the execution of the second prevents the
				// first from being processed. Thus, the first should be
				// processed first.
				tx(a(10, 1), a(10, 3)),
				tx(a(12, 1), a(10, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"self-authorization is not a transaction blocker": {
			[]transaction{
				tx(a(10, 1), a(10, 1), a(12, 3)),
				tx(a(12, 1), a(10, 1), a(10, 1), a(10, 1)), // < would be favoured due to the longer list of authorizations if the self-authorization would be considered a transaction blocker
			},
			[]int{0, 1},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			// make sure the test examples contain connected transactions
			parts := partition(tt.transactions)
			require.LessOrEqual(len(parts), 1)

			got := sortPartition2(tt.transactions, pickHighestPotential)
			res := toIndices(t, tt.transactions, got)
			require.Equal(tt.result, res)
		})
	}
}

func TestSortPartition3_KeyExamples(t *testing.T) {

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
		"identifies authorizations as an enabler": {
			[]transaction{
				tx(a(10, 2)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{1, 0},
		},
		"identifies authorizations as a disabler": {
			[]transaction{
				tx(a(10, 1)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"delays authentications if another transaction could enable it": {
			[]transaction{
				tx(a(10, 1), a(12, 2)),
				tx(a(12, 1)),
			},
			[]int{1, 0},
		},
		"favor longer authentication lists in conflict cases": {
			[]transaction{
				// Only one of these transactions can succeed, while the other
				// one is turned obsolete. In such cases, the one with the
				// longer list of authorizations should be favored.
				tx(a(10, 1), a(12, 1)),
				tx(a(12, 1), a(10, 1), a(14, 2)),
			},
			[]int{1},
		},
		"authentication should not be processed if it blocks a transaction": {
			[]transaction{
				// both are ready, neither has all authorizations ready to be
				// processed, but the execution of the second would prevent the
				// first from being processed. Thus, the first should be
				// processed first.
				tx(a(10, 1), a(10, 3)),
				tx(a(12, 1), a(10, 1)),
			},
			[]int{0, 1},
		},
		"self-authorization is not a transaction blocker": {
			[]transaction{
				tx(a(10, 1), a(10, 2)),
			},
			[]int{0},
		},
		"unreachable pending is not blocking executable transactions": {
			[]transaction{
				tx(a(10, 3)), // < unreachable
				tx(a(12, 1), a(10, 3)),
				tx(a(10, 1), a(12, 1)), // should be after the previous one
			},
			[]int{1, 2},
		},
		"inactive authorizations should be ignored": {
			// The second transaction should not be prevented from being
			// processed first due to its first authorization which, if
			// processed correctly, would invalidate the third transaction.
			// Since initial the account 10 has a nonce of 1, the authorizations
			// of the second transaction have no effect. Thus, it should be
			// processed first.
			[]transaction{
				tx(a(10, 1), a(12, 1)),
				tx(a(12, 1), a(10, 2), a(10, 3)),
				tx(a(10, 2)),
			},
			[]int{1, 0, 2},
		},
		/*
			// Something not supported:
			"deliberately fail authorization to enable additional transaction": {
				[]transaction{
					tx(a(10, 1)),
					tx(a(12, 1), a(10, 2)),
					tx(a(14, 1), a(10, 3)), // < should be processed before the 2nd to avoid blocking the 4th
					tx(a(10, 3), a(14, 1)),
				},
				[]int{0, 2, 1, 3},
			},
		*/
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			// make sure the test examples contain connected transactions
			parts := partition(tt.transactions)
			require.LessOrEqual(len(parts), 1)

			got := sortPartition3(tt.transactions, pickHighestPotential)
			res := toIndices(t, tt.transactions, got)
			require.Equal(tt.result, res)
		})
	}
}

func TestInterleaving_Examples_ProduceExpectedInterleaving(t *testing.T) {
	test := map[string][][]int{
		"empty":                             {},
		"single partition with one element": {{0}},
		"single partition with multiple elements":    {{0, 1, 2}},
		"two partitions with one element each":       {{0}, {1}},
		"two partitions with multiple elements each": {{0, 1, 2}, {3, 4, 5}},
		"two partitions with different lengths":      {{0, 1, 2}, {3}},
		"three partitions with different lengths":    {{0, 1, 2}, {3}, {4, 5}},
	}

	impls := []func([][]int) []int{
		interleavePartitions[int],
		interleavePartitions1[int],
		interleavePartitions2[int],
		interleavePartitions3[int],
	}
	for i, impl := range impls {
		t.Run(fmt.Sprintf("impl-%d", i), func(t *testing.T) {
			for name, partition := range test {
				t.Run(name, func(t *testing.T) {
					require := require.New(t)

					// the test input needs to be copied since the
					// implementation may modify it
					input := make([][]int, len(partition))
					for i := range partition {
						input[i] = slices.Clone(partition[i])
					}

					got := impl(input)

					// make sure that the number of elements is preserved
					total := 0
					for _, part := range partition {
						total += len(part)
					}
					require.Len(got, total)

					// make sure that all elements of the partitions are present
					for _, part := range partition {
						for _, elem := range part {
							require.Contains(got, elem)
						}
					}

					// make sure that the order within the partitions is preserved
					for _, part := range partition {
						for i := range len(part) - 1 {
							a := part[i]
							b := part[i+1]
							require.Less(slices.Index(got, a), slices.Index(got, b))
						}
					}
				})
			}
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

func FuzzGetExecutionOrder_OptimalHeuristicProducesOptimalOrder(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		sorted := GetExecutionOrder(transactions, sortPartition, pickOptimal)

		state := getPresumedInitialState(transactions)
		obtainedScore := eval(state, sorted)

		bestScore := score{}
		bestOrder := []transaction{}
		for permutation := range permute(transactions) {
			cur := eval(state, permutation)
			if cur.isBetterThan(bestScore) {
				bestScore = cur
				bestOrder = slices.Clone(permutation)
			}
		}

		if obtainedScore != bestScore {
			t.Log("transactions:", transactions)
			t.Log("sorted:", sorted)
			t.Log("optimal:", bestOrder)
			t.Fatalf("obtained %v, expected %v", obtainedScore, bestScore)
		}
	})
}

func FuzzGetExecutionOrder_ProducesAFullyExecutableTransactionOrder(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		sorted := GetExecutionOrder(transactions, sortPartition)

		// Compute the initial state of nonces indicated by the transactions.
		state := getPresumedInitialState(transactions)

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
		state := getPresumedInitialState(transactions)
		a := GetExecutionOrder(transactions, sortPartition, pickFirst)
		b := GetExecutionOrder(transactions, sortPartition, pickOptimal)

		scoreA := eval(state.copy(), a)
		scoreB := eval(state.copy(), b)
		if scoreA != scoreB {
			t.Log("transactions: ", transactions)
			t.Log("pickFirst:    ", a)
			t.Log("pickOptimal:  ", b)
			t.Fatalf("different scores: %v vs %v", scoreA, scoreB)
		}
	})
}

func FuzzGetExecutionOrder_FindDiffBetweenOptimalAndPickHighestPotential(f *testing.F) {
	// This fuzzer test helps to identify cases where the optimal and the
	// highest-potential strategy produce different results. This can help to
	// identify edge cases where the heuristic needs to be improved.
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		state := getPresumedInitialState(transactions)
		a := GetExecutionOrder(transactions, sortPartition, pickHighestPotential)
		b := GetExecutionOrder(transactions, sortPartition, pickOptimal)

		scoreA := eval(state.copy(), a)
		scoreB := eval(state.copy(), b)
		if scoreA != scoreB {
			t.Log("transactions:         ", transactions)
			t.Log("pickHighestPotential: ", a)
			t.Log("pickOptimal:          ", b)
			t.Fatalf("different scores: %v vs %v", scoreA, scoreB)
		}
	})
}

func FuzzGetExecutionOrder_FindDiffBetweenOptimalAndSortPartition2(f *testing.F) {
	// This fuzzer test helps to identify cases where the optimal and the
	// highest-potential strategy produce different results. This can help to
	// identify edge cases where the heuristic needs to be improved.
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		state := getPresumedInitialState(transactions)
		a := GetExecutionOrder(transactions, sortPartition2)
		b := GetExecutionOrder(transactions, sortPartition, pickOptimal)

		scoreA := eval(state.copy(), a)
		scoreB := eval(state.copy(), b)
		//if scoreA != scoreB {
		if scoreA.numTransactions != scoreB.numTransactions {
			t.Log("transactions:   ", transactions)
			t.Log("SortPartition2: ", a)
			t.Log("pickOptimal:    ", b)
			t.Fatalf("different scores: %v vs %v", scoreA, scoreB)
		}
	})
}

func FuzzGetExecutionOrder_FindDiffBetweenOptimalAndSortPartition3(f *testing.F) {
	// This fuzzer test helps to identify cases where the optimal and the
	// highest-potential strategy produce different results. This can help to
	// identify edge cases where the heuristic needs to be improved.
	f.Fuzz(func(t *testing.T, data []byte) {
		transactions := parseTransactions(data)
		state := getPresumedInitialState(transactions)
		a := GetExecutionOrder(transactions, sortPartition3)
		b := GetExecutionOrder(transactions, sortPartition, pickOptimal)

		scoreA := eval(state.copy(), a)
		scoreB := eval(state.copy(), b)
		//if scoreA != scoreB {
		if scoreA.numTransactions != scoreB.numTransactions {
			t.Log("transactions:   ", transactions)
			t.Log("SortPartition3: ", a)
			t.Log("pickOptimal:    ", b)
			t.Fatalf("different scores: %v vs %v", scoreA, scoreB)
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

func TestPermute_ProducesAllPermutations(t *testing.T) {
	for numEntries := range 5 {
		t.Run(fmt.Sprintf("N=%d", numEntries), func(t *testing.T) {
			require := require.New(t)

			elements := make([]int, numEntries)
			for i := range elements {
				elements[i] = i
			}

			// Compute all permutations.
			permutations := slices.Collect(permute(elements))

			// Check that the number of permutations is correct.
			want := 1
			for i := 1; i <= numEntries; i++ {
				want *= i
			}
			require.Equal(want, len(permutations))

			// check that all permutations contain the same elements
			for _, permutation := range permutations {
				require.ElementsMatch(elements, permutation)
			}

			// Check that all permutations are unique.
			unique := map[string]bool{}
			for _, permutation := range permutations {
				key := fmt.Sprint(permutation)
				_, found := unique[key]
				require.False(found, "duplicate permutation: %v", permutation)
				unique[key] = true
			}
		})
	}
}

func permute[T any](elements []T) iter.Seq[[]T] {
	// An implementation of the Heap's algorithm to generate all permutations of
	// the given elements.
	// See https://en.wikipedia.org/wiki/Heap%27s_algorithm
	return func(yield func([]T) bool) {
		if !yield(slices.Clone(elements)) {
			return
		}

		list := slices.Clone(elements)
		c := make([]int, len(list))
		i := 1
		for i < len(list) {
			if c[i] < i {
				if i%2 == 0 {
					list[0], list[i] = list[i], list[0]
				} else {
					list[c[i]], list[i] = list[i], list[c[i]]
				}
				if !yield(slices.Clone(list)) {
					return
				}
				c[i]++
				i = 1
			} else {
				c[i] = 0
				i++
			}
		}
	}
}
