package inter

import (
	"testing"

	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
	"github.com/stretchr/testify/require"
)

func TestGetProposer_IsDeterministic(t *testing.T) {
	require := require.New(t)

	builder := pos.ValidatorsBuilder{}
	builder.Set(1, 10)
	builder.Set(2, 20)
	builder.Set(3, 30)
	validators := builder.Build()

	for i := range 5 {
		for j := range uint32(5) {
			a, err := GetProposer(validators, idx.Block(i), j)
			require.NoError(err)
			b, err := GetProposer(validators, idx.Block(i), j)
			require.NoError(err)
			require.Equal(a, b, "proposer selection is not deterministic")
		}
	}
}

func TestGetProposer_ProposersAreSelectedProportionalToStake(t *testing.T) {
	require := require.New(t)

	builder := pos.ValidatorsBuilder{}
	builder.Set(1, 10)
	builder.Set(2, 20)
	builder.Set(3, 30)
	builder.Set(4, 40)
	validators := builder.Build()

	tests := map[string]func(i int) (idx.ValidatorID, error){
		"over blocks": func(i int) (idx.ValidatorID, error) {
			return GetProposer(validators, idx.Block(i), 0)
		},
		"over attempts": func(i int) (idx.ValidatorID, error) {
			return GetProposer(validators, 1, uint32(i))
		},
		"mixed blocks and attempts": func(i int) (idx.ValidatorID, error) {
			return GetProposer(validators, idx.Block(i/5), uint32(i%5))
		},
	}

	for name, sample := range tests {
		t.Run(name, func(t *testing.T) {
			const Samples = 100000
			counters := map[idx.ValidatorID]int{}
			for i := range Samples {
				proposer, err := sample(i)
				require.NoError(err)
				counters[proposer]++
			}

			tolerance := float64(Samples / 100) // 1% tolerance
			for id, idx := range validators.Idxs() {
				expected := int(Samples * validators.GetWeightByIdx(idx) / validators.TotalWeight())
				require.InDelta(
					counters[id], expected, tolerance,
					"validator %d is not selected proportional to stake", id,
				)
			}
		})
	}
}
