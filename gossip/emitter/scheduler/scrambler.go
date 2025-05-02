package scheduler

import (
	"math/rand/v2"
	"slices"

	scramblerImpl "github.com/0xsoniclabs/sonic/gossip/scrambler"
	"github.com/ethereum/go-ethereum/core/types"
)

//go:generate mockgen -source=scrambler.go -destination=scrambler_mock.go -package=scheduler

// scrambler is an internal interface for a component handling the scrambling of
// a transaction schedule.
type scrambler interface {
	// scramble returns a random permutation of the given transactions. The
	// input slice must not be modified.
	scramble(transactions []*types.Transaction, signer types.Signer, seed uint64) []*types.Transaction
}

type prototypeScrambler struct{}

// scramble returns a random permutation of the given transactions. The result
// is a copy of the input slice, so the input is not modified.
func (prototypeScrambler) scramble(transactions []*types.Transaction, _ types.Signer, _ uint64) []*types.Transaction {
	// TODO: this is a proto-type implementation and needs to be replaced by
	// a verifiable shuffling algorithm (issue #159).
	res := slices.Clone(transactions)
	rand.Shuffle(len(res), func(i, j int) {
		res[i], res[j] = res[j], res[i]
	})
	return res
}

type allegroScrambler struct{}

func (allegroScrambler) scramble(transactions []*types.Transaction, signer types.Signer, seed uint64) []*types.Transaction {
	scrambledTransactions := scramblerImpl.Scramble(transactions, seed, signer)
	return scrambledTransactions
}
