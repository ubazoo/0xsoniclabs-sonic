package scrambler

import (
	"cmp"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/0xsoniclabs/sonic/gossip/emitter/mock"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestScrambler_EndToEnd(t *testing.T) {
	numTransactions := 256
	signer := types.NewPragueSigner(big.NewInt(1))

	// generate numTransactions account keys
	accountKeys := make([]*ecdsa.PrivateKey, numTransactions)
	for i := range numTransactions {
		key, err := crypto.GenerateKey()
		require.NoError(t, err)
		accountKeys[i] = key
	}

	transactions := make([]*types.Transaction, 0, numTransactions)
	for i := range numTransactions {
		key := accountKeys[i]
		tx, err := types.SignTx(types.NewTx(&types.LegacyTx{
			Nonce:    uint64(i),
			GasPrice: big.NewInt(int64(42 + i)),
		}), signer, key)
		require.NoError(t, err)
		transactions = append(transactions, tx)
	}

	transactions = Scramble(transactions, 42, signer)

	// Check that the transactions are not lost during conversion and scrambling
	if len(transactions) != numTransactions {
		t.Fatalf("expected %d transactions, got %d", numTransactions, len(transactions))
	}

	// Check that the transactions are not duplicated
	seenHashes := make(map[common.Hash]struct{})
	for _, tx := range transactions {
		if _, ok := seenHashes[tx.Hash()]; ok {
			t.Fatalf("transaction %s is duplicated", tx.Hash().Hex())
		}
		seenHashes[tx.Hash()] = struct{}{}
	}

	// Check that scrambling is deterministic
	transactions2 := Scramble(transactions, 42, signer)
	require.Equal(t, transactions, transactions2, "scrambling should be deterministic")
}

func TestScrambler_DecodingError(t *testing.T) {
	input := types.Transactions{
		types.NewTx(&types.LegacyTx{
			Nonce: 0,
			Gas:   0,
		}),
		types.NewTx(&types.LegacyTx{
			Nonce: 1,
			Gas:   0,
		}),
		types.NewTx(&types.LegacyTx{
			Nonce: 2,
			Gas:   0,
		}),
		types.NewTx(&types.LegacyTx{
			Nonce: 3,
			Gas:   0,
		}),
	}

	ctrl := gomock.NewController(t)
	signer := mock.NewMockTxSigner(ctrl)

	gomock.InOrder(
		signer.EXPECT().Sender(input[0]).Return(common.Address{1}, nil),
		signer.EXPECT().Sender(input[1]).Return(common.Address{2}, nil),
		signer.EXPECT().Sender(input[2]).Return(common.Address{}, errors.New("error")),
		signer.EXPECT().Sender(input[3]).Return(common.Address{3}, nil),
	)

	output := Scramble(input, 42, signer)
	if len(output) != 3 {
		t.Fatalf("expected 3 transactions, got %d", len(output))
	}
}

func TestScrambler_ScramblingIsDeterministic(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{
			hash:     common.Hash{1},
			sender:   common.Address{1},
			nonce:    1,
			gasPrice: big.NewInt(2),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{2},
			sender:   common.Address{1},
			nonce:    2,
			gasPrice: big.NewInt(1),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{3},
			sender:   common.Address{2},
			nonce:    1,
			gasPrice: big.NewInt(2),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{4},
			sender:   common.Address{2},
			nonce:    2,
			gasPrice: big.NewInt(1),
		},
	}

	permutation1 := scramblePermutation(entries, 42)
	scrambledEntries := reorderTransactions(entries, permutation1)
	for range 100 {
		shuffleEntries(entries)
		permutation2 := scramblePermutation(entries, 42)
		entries = reorderTransactions(entries, permutation2)
		require.Equal(t, scrambledEntries, entries, "scrambling should be deterministic")
	}
}

func TestScrambler_ScramblingIsDeterministicRandomInput(t *testing.T) {
	entries := generateScramblerInput(1000)

	permutation1 := scramblePermutation(entries, 42)
	scrambledEntries := reorderTransactions(entries, permutation1)
	for range 10 {
		shuffleEntries(entries)
		permutation2 := scramblePermutation(entries, 42)
		entries = reorderTransactions(entries, permutation2)
		require.Equal(t, scrambledEntries, entries, "scrambling should be deterministic")
	}
}

func TestScrambler_OrderIsBasedOnNonceGasPriceAndHash(t *testing.T) {
	entries := generateScramblerInput(10000)

	permutation1 := scramblePermutation(entries, 42)
	scrambledEntries := reorderTransactions(entries, permutation1)

	previous := entries[0]
	previousSender := previous.Sender()
	for i, entry := range scrambledEntries {
		if i == 0 {
			continue
		}
		if previousSender.Cmp(entry.Sender()) != 0 {
			previous = entry
			previousSender = entry.Sender()
			continue
		}
		if cmp.Compare(previous.Nonce(), entry.Nonce()) < 0 {
			previous = entry
			continue
		}
		if previous.GasPrice().Cmp(entry.GasPrice()) > 0 {
			previous = entry
			continue
		}
		if previous.Hash().Cmp(entry.Hash()) < 0 {
			previous = entry
			continue
		}
		t.Fatal("order is not based on sender nonce gas price and hash")
	}
}

func TestScrambler_IsAllegroScrambledCheckSameSender(t *testing.T) {
	entries := []ScramblerEntry{
		// Ordered by nonce
		&dummyScramblerEntry{
			nonce: 1,
		},
		&dummyScramblerEntry{
			nonce: 2,
		},

		// Ordered by gas price
		&dummyScramblerEntry{
			nonce:    3,
			gasPrice: big.NewInt(2),
		},
		&dummyScramblerEntry{
			nonce:    3,
			gasPrice: big.NewInt(1),
		},

		// Ordered by hash
		&dummyScramblerEntry{
			nonce: 4,
			hash:  common.Hash{5},
		},
		&dummyScramblerEntry{
			nonce: 4,
			hash:  common.Hash{6},
		},
	}

	if !isScrambledAllegro(entries, 42) {
		t.Fatal("entries should be in the right scrambled order")
	}

	for i := 0; i < len(entries); i += 2 {
		entries[i], entries[i+1] = entries[i+1], entries[i]
		if isScrambledAllegro(entries, 42) {
			t.Fatal("entries should not be in the right scrambled order")
		}
		entries[i], entries[i+1] = entries[i+1], entries[i]
	}
}

func TestScrambler_IsAllegroScrambledCheckDifferentSenders(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{
			sender: common.Address{1},
		},
		&dummyScramblerEntry{
			sender: common.Address{2},
		},
		&dummyScramblerEntry{
			sender: common.Address{3},
		},
		&dummyScramblerEntry{
			sender: common.Address{4},
		},
		&dummyScramblerEntry{
			sender: common.Address{5},
		},
	}

	scrambleEntries(entries, 42)

	if !isScrambledAllegro(entries, 42) {
		t.Fatal("entries should be in the right scrambled order")
	}
	for i := range len(entries) - 1 {
		entries[i], entries[i+1] = entries[i+1], entries[i]
		if isScrambledAllegro(entries, 42) {
			t.Fatal("entries should not be in the right scrambled order")
		}
		entries[i], entries[i+1] = entries[i+1], entries[i]
	}
}

func TestScrambler_IsAllegroScrambledCheckInterleaved(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{
			sender:   common.Address{1},
			nonce:    1,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			sender:   common.Address{1},
			nonce:    2,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			sender:   common.Address{2},
			nonce:    1,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			sender:   common.Address{2},
			nonce:    2,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			sender:   common.Address{2},
			nonce:    2,
			gasPrice: big.NewInt(1),
		},
	}

	permutation := scramblePermutation(entries, 42)
	entries = reorderTransactions(entries, permutation)

	if !isScrambledAllegro(entries, 42) {
		t.Fatal("entries should be in the right scrambled order")
	}
	for i := range len(entries) - 1 {
		entries[i], entries[i+1] = entries[i+1], entries[i]
		if isScrambledAllegro(entries, 42) {
			t.Fatal("entries should not be in the right scrambled order")
		}
		entries[i], entries[i+1] = entries[i+1], entries[i]
	}
}

func TestScrambler_transactionSendersReturnAllUniqueSenders(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{
			sender: common.Address{1},
		},
		&dummyScramblerEntry{
			sender: common.Address{2},
		},
		&dummyScramblerEntry{
			sender: common.Address{3},
		},
	}

	senders := transactionSenders(entries)
	require.Len(t, senders, 3, "should return all unique senders")
	for i, entry := range entries {
		require.Equal(t, entry.Sender(), senders[i], "should return the same sender")
	}

	entries = append(entries, []ScramblerEntry{
		&dummyScramblerEntry{
			sender: common.Address{1},
		},
		&dummyScramblerEntry{
			sender: common.Address{2},
		},
	}...)
	senders = transactionSenders(entries)
	require.Len(t, senders, 3, "should return all unique senders")
	for i, sender := range senders {
		require.Equal(t, sender, senders[i], "should return the same sender")
	}
}

func TestScrambler_isNextSender(t *testing.T) {
	tests := map[string]struct {
		current common.Address
		next    common.Address
		isNext  bool
	}{
		"correct order": {
			current: common.Address{1},
			next:    common.Address{2},
			isNext:  true,
		},
		"incorrect order": {
			current: common.Address{2},
			next:    common.Address{1},
			isNext:  false,
		},
		"same sender": {
			current: common.Address{1},
			next:    common.Address{1},
			isNext:  false,
		},
		"current not in the list": {
			current: common.Address{5},
			next:    common.Address{1},
			isNext:  false,
		},
		"next not in the list": {
			current: common.Address{1},
			next:    common.Address{5},
			isNext:  false,
		},
		"last sender": {
			current: common.Address{4},
			next:    common.Address{1},
			isNext:  false,
		},
	}

	senders := []common.Address{{1}, {2}, {3}, {4}}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			isNext := isNextSender(test.current, test.next, senders)
			require.Equal(t, test.isNext, isNext, "should return the correct result")
		})
	}
}

func TestScrambler_RoundTrip(t *testing.T) {
	entries := []ScramblerEntry{
		&dummyScramblerEntry{
			hash:     common.Hash{1},
			sender:   common.Address{1},
			nonce:    1,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{2},
			sender:   common.Address{1},
			nonce:    2,
			gasPrice: big.NewInt(0),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{3},
			sender:   common.Address{1},
			nonce:    2,
			gasPrice: big.NewInt(1),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{4},
			sender:   common.Address{1},
			nonce:    2,
			gasPrice: big.NewInt(1),
		},
		&dummyScramblerEntry{
			hash:     common.Hash{5},
			sender:   common.Address{2},
			nonce:    1,
			gasPrice: big.NewInt(1),
		},
	}

	permutation1 := scramblePermutation(entries, 42)
	scrambledEntries := reorderTransactions(entries, permutation1)

	if !isScrambledAllegro(scrambledEntries, 42) {
		t.Fatal("entries should be in the right scrambled order")
	}
}

func TestScrambler_RoundTripRandom(t *testing.T) {
	entries := generateScramblerInput(100000)

	permutation1 := scramblePermutation(entries, 42)
	scrambledEntries := reorderTransactions(entries, permutation1)

	if !isScrambledAllegro(scrambledEntries, 42) {
		t.Fatal("entries should be in the right scrambled order")
	}
}

func TestScrambler_ScrambleEntriesIsDeterministic(t *testing.T) {
	entries := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	reference := slices.Clone(entries)
	scrambleEntries(reference, 42)

	for range 100 {
		scrambled := slices.Clone(entries)
		scrambleEntries(scrambled, 42)

		require.Equal(t, reference, scrambled, "scrambling should be deterministic")
	}
}

func TestScrambler_ScrambleEntriesReturnsDifferentOrders(t *testing.T) {
	entries := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	for seed := range 10 {
		scrambled := slices.Clone(entries)
		scrambleEntries(scrambled, uint64(seed+1))

		require.NotEqual(t, entries, scrambled, "scrambling should return different orders")
	}
}

func TestScrambler_ReorderTransactions(t *testing.T) {
	items := make([]int, 1000)
	for i := range items {
		items[i] = i
	}

	indices := rand.Perm(len(items))
	items = reorderTransactions(items, indices)
	require.Equal(t, indices, items, "permutation should be the same as the original items")
}

func generateScramblerInput(size int) []ScramblerEntry {
	entries := make([]ScramblerEntry, size)
	for i := range size {
		// ~1/10th of the entries will have the same sender, nonce or gas price
		sender := rand.IntN(size / 10)
		nonce := rand.IntN(size / 10)
		gasPrice := rand.IntN(size / 10)
		entries[i] = &dummyScramblerEntry{
			hash:     common.Hash(uint256.NewInt(uint64(i)).Bytes32()),
			sender:   common.Address(uint256.NewInt(uint64(sender)).Bytes20()),
			nonce:    uint64(nonce),
			gasPrice: big.NewInt(int64(gasPrice)),
		}
	}

	return entries
}
