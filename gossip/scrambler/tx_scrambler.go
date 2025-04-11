package scrambler

import (
	"cmp"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/core/types"
)

// ScramblerEntry stores meta information about transaction for sorting and filtering them.
type ScramblerEntry interface {
	// Hash returns the transaction hash
	Hash() common.Hash
	// Sender returns the sender of the transaction
	Sender() common.Address
	// Nonce returns the transaction nonce
	Nonce() uint64
	// GasPrice returns the transaction gas price
	GasPrice() *big.Int
}

type scramblerTransaction struct {
	*types.Transaction
	sender common.Address
}

func (tx *scramblerTransaction) Sender() common.Address {
	return tx.sender
}

func Scramble(seed uint64, signer types.Signer, transactions []*types.Transaction) []*types.Transaction {
	inputTransactions := make([]ScramblerEntry, 0, len(transactions))
	for _, tx := range transactions {
		sender, err := types.Sender(signer, tx)
		if err != nil {
			continue
		}

		entry := &scramblerTransaction{
			Transaction: tx,
			sender:      sender,
		}
		inputTransactions = append(inputTransactions, entry)
	}

	orderedEntries := scrambleTransactions(seed, inputTransactions)

	orderedTxs := make(types.Transactions, len(orderedEntries))
	for i, tx := range orderedEntries {
		// Cast back the transactions to pass it to the processor
		orderedTxs[i] = tx.(*scramblerTransaction).Transaction
	}

	return orderedTxs
}

func scrambleTransactions(seed uint64, transactions []ScramblerEntry) []ScramblerEntry {
	// Group transactions by sender
	sendersTransactions := map[common.Address][]ScramblerEntry{}
	for _, tx := range transactions {
		sendersTransactions[tx.Sender()] = append(sendersTransactions[tx.Sender()], tx)
	}

	// Sort transactions by nonce, gas price and hash
	for _, txs := range sendersTransactions {
		// Sort by nonce
		slices.SortFunc(txs, func(a, b ScramblerEntry) int {
			res := cmp.Compare(a.Nonce(), b.Nonce())
			if res != 0 {
				return res
			}
			// if nonce is same, sort by gas price
			res = b.GasPrice().Cmp(a.GasPrice())
			if res != 0 {
				return res
			}
			return a.Hash().Cmp(b.Hash())
		})
	}

	// Scramble senders
	senders := make([]common.Address, 0, len(sendersTransactions))
	for sender := range sendersTransactions {
		senders = append(senders, sender)
	}
	// Sort senders by address, so that the shuffle is deterministic
	slices.SortFunc(senders, func(a, b common.Address) int {
		return a.Cmp(b)
	})
	senders = shuffleSenders(seed, senders)

	scrambledTransactions := make([]ScramblerEntry, 0, len(transactions))
	for _, sender := range senders {
		scrambledTransactions = append(scrambledTransactions, sendersTransactions[sender]...)
	}

	return scrambledTransactions
}

func shuffleSenders[T any](seed uint64, senders []T) []T {
	rand := xorShiftStar{}
	rand.seed(seed)

	// This is an implementation of the Fisher-Yates shuffle algorithm.
	// See: https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
	for i := len(senders) - 1; i > 0; i-- {
		j := rand.randN(uint64(i + 1))
		senders[i], senders[j] = senders[j], senders[i]
	}

	return senders
}

type xorShiftStar struct {
	state uint64
}

// seed initializes the xorShiftStar random number generator with the given seed.
// seed must be non-zero.
func (x *xorShiftStar) seed(seed uint64) {
	x.state = seed
}

// next returns the next random number using the xorshift* algorithm.
// Based on https://en.wikipedia.org/wiki/Xorshift#xorshift*
func (x *xorShiftStar) next() uint64 {
	const factor = uint64(0x2545F4914F6CDD1D)
	rand := x.state
	rand ^= rand >> 12
	rand ^= rand << 25
	rand ^= rand >> 27
	x.state = rand
	return rand * factor
}

// randN returns a random number in the range [0, n) using the xorshift* algorithm.
// it removes the modulo bias
func (x *xorShiftStar) randN(n uint64) uint64 {
	uint64Max := ^uint64(0)
	var rand uint64
	for {
		rand = x.next()
		// for small n, this will most likely be true in the first iteration
		if rand < (uint64Max - (uint64Max % n)) {
			break
		}
	}
	return rand % n
}
