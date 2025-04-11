package scrambler

import (
	"cmp"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
)

// dummyScramblerEntry represents scramblery entry data used for testing
type dummyScramblerEntry struct {
	hash     common.Hash    // transaction hash
	sender   common.Address // sender of the transaction
	nonce    uint64         // transaction nonce
	gasPrice *big.Int       // transaction gasPrice
}

func (s *dummyScramblerEntry) Hash() common.Hash {
	return s.hash
}

func (s *dummyScramblerEntry) Sender() common.Address {
	return s.sender
}

func (s *dummyScramblerEntry) Nonce() uint64 {
	return s.nonce
}

func (s *dummyScramblerEntry) GasPrice() *big.Int {
	return s.gasPrice
}

func compareFunc(a ScramblerEntry, b ScramblerEntry) int {
	addrCmp := a.Sender().Cmp(b.Sender())
	if addrCmp != 0 {
		return addrCmp
	}
	res := cmp.Compare(a.Nonce(), b.Nonce())
	if res != 0 {
		return res
	}
	res = a.GasPrice().Cmp(b.GasPrice())
	if res != 0 {
		return res
	}
	return a.Hash().Cmp(b.Hash())
}

func TestScrambler_(t *testing.T) {

}

func TestTxScrambler_ScrambleTransactions_ScrambleIsDeterministic(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{sender: common.Address{1}, hash: common.Hash{1}},
		&dummyScramblerEntry{sender: common.Address{2}, hash: common.Hash{2}},
		&dummyScramblerEntry{sender: common.Address{3}, hash: common.Hash{3}},
		&dummyScramblerEntry{sender: common.Address{4}, hash: common.Hash{4}},
	}

	entriesCopy := slices.Clone(entries)

	for range 10 {
		entries = scrambleTransactions(42, entries)
		for range 10 {
			rand.Shuffle(len(entriesCopy), func(i, j int) {
				entriesCopy[i], entriesCopy[j] = entriesCopy[j], entriesCopy[i]
			})
			entriesCopy = scrambleTransactions(42, entriesCopy)
			if slices.CompareFunc(entries, entriesCopy, compareFunc) != 0 {
				t.Fatalf("scramble is not deterministic %v vs %v", entries, entriesCopy)
			}
		}
	}
}

func TestPseudoRandomNumberGenerator_OutputIsEquidistributed(t *testing.T) {
	rnd := xorShiftStar{}
	rnd.seed(42)
	n := 1000000

	seenXor := map[uint64]struct{}{}
	seenRand := map[uint64]struct{}{}

	for range n {
		seenXor[rnd.randN(uint64(n))] = struct{}{}
		seenRand[rnd.randN(uint64(n))] = struct{}{}
	}

	if len(seenXor) < len(seenRand)*99/100 {
		t.Fatalf("Homemade rand is not within 1 percent of rand library: %d vs. %d", len(seenXor), len(seenRand))
	}
}
func TestScrambler_shuffleSendersReturnsDifferentPermutations(t *testing.T) {
	n := 8
	numPermutations := fac(n)
	seeds := make([]int, numPermutations)
	for i := range seeds {
		seeds[i] = i
	}

	permutations := map[string]struct{}{}
	for _, seed := range seeds {
		input := []rune{}
		for i := range n {
			input = append(input, rune('A'+i))
		}
		out := shuffleSenders(uint64(seed), input)
		permutations[string(out)] = struct{}{}
	}

	if len(permutations) < numPermutations/2 {
		t.Fatalf("Missing permutations, only got %d of %d: %v", len(permutations), numPermutations, permutations)
	}
}

func fac(n int) int {
	if n == 0 {
		return 1
	}
	return n * fac(n-1)
}
