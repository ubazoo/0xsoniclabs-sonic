package scheduler

import (
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestPrototypeScrambler_DoesNotAlterTheInput(t *testing.T) {
	input := []*types.Transaction{
		types.NewTx(&types.LegacyTx{Nonce: 1}),
		types.NewTx(&types.LegacyTx{Nonce: 2}),
		types.NewTx(&types.LegacyTx{Nonce: 3}),
	}
	backup := slices.Clone(input)

	scrambler := prototypeScrambler{}
	scrambler.scramble(input)
	require.Equal(t, backup, input)
}

func TestPrototypeScrambler_OutputContainsSameElementsAsInput(t *testing.T) {
	input := []*types.Transaction{
		types.NewTx(&types.LegacyTx{Nonce: 1}),
		types.NewTx(&types.LegacyTx{Nonce: 2}),
		types.NewTx(&types.LegacyTx{Nonce: 3}),
	}

	scrambler := prototypeScrambler{}
	for i := range len(input) {
		output := scrambler.scramble(input[:i])
		require.ElementsMatch(t, input[:i], output)
	}
}
