package scrambler

import (
	"cmp"
	"maps"
	"slices"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/core/types"
)

// Scramble takes a list of transactions and a seed and scrambles the input
// list of transactions. The scrambling is done in a way that is deterministic
// and can be reproduced by using the same seed. The transactions are grouped
// by sender and sorted by nonce, gas price and hash. The senders are then
// shuffled using the seed and the transactions are reordered according to
// the shuffled senders.
func Scramble(transactions []*types.Transaction, seed uint64, signer types.Signer) []*types.Transaction {
	// Convert transactions to scrambler entries
	entries := convertToScramblerEntry(transactions, signer)

	// Get scrambled order
	permutation := scramblePermutation(entries, seed)

	// Apply permutation to transactions
	return reorderTransactions(transactions, permutation)
}

// IsScrambledAllegro checks if the transactions are in the correct order
// according to the allegro scrambling. The transactions need to be grouped by
// sender and sorted by nonce, gas price and hash.
func IsScrambledAllegro(entries []*types.Transaction, seed uint64, signer types.Signer) bool {
	// Convert transactions to scrambler entries
	scramblerEntries := convertToScramblerEntry(entries, signer)

	// Check if the order of the entries is correct
	return isScrambledAllegro(scramblerEntries, seed)
}

// convertToScramblerEntry converts a list of transactions to a list of
// scrambler entries. The scrambler entry contains the fields required for
// scrambling the transactions.
func convertToScramblerEntry(transactions []*types.Transaction, signer types.Signer) []ScramblerEntry {
	entries := make([]ScramblerEntry, 0, len(transactions))
	for _, tx := range transactions {
		sender, err := types.Sender(signer, tx)
		if err != nil {
			continue
		}

		entry := &scramblerTransaction{
			Transaction: tx,
			sender:      sender,
		}
		entries = append(entries, entry)
	}
	return entries
}

// scramblePermutation takes a seed and a list of transactions and returns
// a permutation of the transactions. The permutation is done in a way that
// it is deterministic and can be reproduced by using the same seed.
func scramblePermutation(transactions []ScramblerEntry, seed uint64) []int {
	// Group transactions by sender
	sendersTransactions := map[common.Address][]int{}
	for idx, tx := range transactions {
		sendersTransactions[tx.Sender()] = append(sendersTransactions[tx.Sender()], idx)
	}

	// Sort transactions by nonce, gas price and hash
	for _, txs := range sendersTransactions {
		// Sort by nonce
		slices.SortFunc(txs, func(idxA, idxB int) int {
			a := transactions[idxA]
			b := transactions[idxB]
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

	senders := slices.Collect(maps.Keys(sendersTransactions))
	// Sort senders by address, so that the shuffle is deterministic
	slices.SortFunc(senders, func(a, b common.Address) int {
		return a.Cmp(b)
	})
	// Shuffle senders
	senders = scrambleEntries(senders, seed)

	// Save permutation of transactions
	permutation := make([]int, 0, len(transactions))
	for _, sender := range senders {
		permutation = append(permutation, sendersTransactions[sender]...)
	}

	return permutation
}

// scrambleEntries shuffles the entries in place using the given seed.
// It uses the Fisher-Yates shuffle algorithm to ensure that the shuffle is
// uniform and unbiased. https://en.wikipedia.org/wiki/Fisher%E2%80%93Yates_shuffle
func scrambleEntries[T any](entries []T, seed uint64) []T {
	randomGenerator := randomGenerator{}
	randomGenerator.seed(seed)
	for i := len(entries) - 1; i > 0; i-- {
		j := randomGenerator.randN(uint64(i + 1))
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries
}

// reorderTransactions takes a list of transactions and a permutation and
// returns a new list of transactions in the order specified by the permutation.
func reorderTransactions[T any](entries []T, permutation []int) []T {
	scrambledTransactions := make([]T, len(permutation))
	for i, idx := range permutation {
		scrambledTransactions[i] = entries[idx]
	}
	return scrambledTransactions
}

// isScrambledAllegro checks if the entries are in the correct order according
// to the allegro scrambling.
func isScrambledAllegro(entries []ScramblerEntry, seed uint64) bool {
	if len(entries) == 0 {
		return true
	}

	senders := transactionSenders(entries)
	senders = scrambleEntries(senders, seed)

	previous := entries[0]

	// Check if the order of the entries is correct
	for i, tx := range entries {
		if i == 0 {
			continue
		}
		// If sender is not the same as previous, check if it is the next
		// in order otherwise the order is not correct
		if tx.Sender().Cmp(previous.Sender()) != 0 {
			if isNextSender(previous.Sender(), tx.Sender(), senders) {
				previous = tx
				continue
			} else {
				return false
			}
		}

		// The same sender as the previous, check if the nonce is correct
		if cmp.Compare(tx.Nonce(), previous.Nonce()) < 0 {
			return false
		}
		// If the nonce is increasing, no need to check the gas price
		if cmp.Compare(tx.Nonce(), previous.Nonce()) > 0 {
			previous = tx
			continue
		}

		// The same sender and nonce, check if the gas price is correct
		if tx.GasPrice().Cmp(previous.GasPrice()) > 0 {
			return false
		}
		// If the gas price is decreasing, no need to check the hash
		if tx.GasPrice().Cmp(previous.GasPrice()) < 0 {
			previous = tx
			continue
		}

		// The same sender, nonce and gas price, check if the hash is correct
		if tx.Hash().Cmp(previous.Hash()) > 0 {
			previous = tx
			continue
		}
		return false
	}

	return true
}

// transactionSenders returns an ordered list of unique senders from the given entries.
func transactionSenders(entries []ScramblerEntry) []common.Address {
	senderMap := make(map[common.Address]struct{})
	for _, entry := range entries {
		senderMap[entry.Sender()] = struct{}{}
	}
	senders := slices.Collect(maps.Keys(senderMap))
	slices.SortFunc(senders, func(a, b common.Address) int {
		return a.Cmp(b)
	})
	return senders
}

// isNextSender checks if the next sender is the next in the list of senders.
func isNextSender(current, next common.Address, senders []common.Address) bool {
	for i := range senders {
		if current.Cmp(senders[i]) == 0 {
			if i+1 < len(senders) && next.Cmp(senders[i+1]) == 0 {
				return true
			}
			return false
		}
	}
	return false
}
