package scheduler

import (
	"context"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// TODO:
// - test that scheduler is incrementally increasing the search-window

func TestScheduler_Schedule_ScramblesTransactionsBeforeEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	scrambler := NewMockscrambler(ctrl)
	evaluator := NewMockevaluator(ctrl)

	// The following mocks are configured to test that the data produces by the
	// scrambler is passed to the evaluator.
	const N = 5

	any := gomock.Any()
	counter := uint64(0)
	scrambler.EXPECT().scramble(any).DoAndReturn(
		func(_ []*types.Transaction) []*types.Transaction {
			counter++
			return []*types.Transaction{
				types.NewTx(&types.LegacyTx{Nonce: counter}),
			}
		},
	).Times(N)

	evaluator.EXPECT().evaluate(any, any, any, any).DoAndReturn(
		func(_ context.Context, _ *BlockInfo, txs []*types.Transaction, _ uint64) ([]*types.Transaction, uint64, bool) {
			require.Equal(t, counter, txs[0].Nonce())
			return nil, counter, false
		},
	).Times(N)

	txs := []*types.Transaction{}
	for range 1 << (N - 1) {
		txs = append(txs, types.NewTx(&types.LegacyTx{Nonce: 0}))
	}

	newScheduler(scrambler, evaluator).Schedule(
		t.Context(),
		&BlockInfo{},
		slices.Values(txs),
		100_000_000,
	)
}

func TestScheduler_Schedule_SelectsTheResultWithTheBestEvaluation(t *testing.T) {
	ctrl := gomock.NewController(t)
	scrambler := &prototypeScrambler{}
	evaluator := NewMockevaluator(ctrl)

	// The evaluator picks the 3rd input as the best one, whatever it is.
	any := gomock.Any()
	counter := 0
	var best []*types.Transaction
	evaluator.EXPECT().evaluate(any, any, any, any).DoAndReturn(
		func(_ context.Context, _ *BlockInfo, txs []*types.Transaction, _ uint64) ([]*types.Transaction, uint64, bool) {
			counter++
			if counter == 3 {
				best = slices.Clone(txs)
				return txs, 1, false // uses 1 gas
			}
			return nil, 0, false // all others use 0 gas
		},
	).MinTimes(4)

	txs := []*types.Transaction{}
	for range 32 {
		txs = append(txs, types.NewTx(&types.LegacyTx{Nonce: 0}))
	}

	result := newScheduler(scrambler, evaluator).Schedule(
		t.Context(),
		&BlockInfo{},
		slices.Values(txs),
		100_000_000,
	)

	require.Equal(t, best, result)
}

func TestScheduler_Schedule_FavorsTransactionsWithTheHighestPriority(t *testing.T) {
	ctrl := gomock.NewController(t)
	scrambler := &prototypeScrambler{}

	// Create an evaluator that accepts everything with a constant gas cost.
	const perTxGasCosts = 2
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)
	factory.EXPECT().BeginBlock(gomock.Any()).Return(processor).AnyTimes()
	processor.EXPECT().Release().AnyTimes()
	processor.EXPECT().Run(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ *types.Transaction, remainingGas uint64) (runResult, uint64) {
			if remainingGas < perTxGasCosts {
				return runOutOfGas, 0
			}
			return runSuccess, perTxGasCosts
		},
	).AnyTimes()
	evaluator := &executingEvaluator{factory}

	// Create a list of transactions to be scheduled. The order defines the
	// relative priority of the transactions.
	const N = 32
	txs := []*types.Transaction{}
	for i := range N {
		txs = append(txs, types.NewTx(&types.LegacyTx{Nonce: uint64(i)}))
	}

	// Test that for any gas limit the transactions with the highest priority
	// are selected. Note: priority is defined by the input order.
	scheduler := newScheduler(scrambler, evaluator)
	for limit := range perTxGasCosts*N + 2 {
		result := scheduler.Schedule(
			t.Context(),
			&BlockInfo{},
			slices.Values(txs),
			uint64(limit),
		)
		require.Equal(t, limit/perTxGasCosts, len(result))
		require.ElementsMatch(t, txs[:len(result)], result)
	}
}

func TestScheduler_Schedule_CanBeCanceled(t *testing.T) {
	ctrl := gomock.NewController(t)
	scrambler := &prototypeScrambler{}

	const N = 32
	txs := []*types.Transaction{}
	for i := range N {
		txs = append(txs, types.NewTx(&types.LegacyTx{Nonce: uint64(i)}))
	}

	for numEvaluationsBeforeCancel := range 10 {
		evaluator := NewMockevaluator(ctrl)
		ctxt, cancel := context.WithCancel(t.Context())

		// Cancel the scheduling after a given number of evaluations.
		counter := 0
		any := gomock.Any()
		evaluator.EXPECT().evaluate(any, any, any, any).DoAndReturn(
			func(_ context.Context, _ *BlockInfo, txs []*types.Transaction, _ uint64) (
				[]*types.Transaction, uint64, bool,
			) {
				require.LessOrEqual(t, counter, numEvaluationsBeforeCancel)
				if counter == numEvaluationsBeforeCancel {
					cancel()
				}
				counter++

				// To cover the initial exponential growth phase and the binary
				// search step, this emitter will accept at most the first 4
				// transactions.
				res := txs
				limitReached := false
				if len(res) > 4 {
					res = res[:4]
					limitReached = true
				}
				return res, uint64(len(res)), limitReached
			},
		).AnyTimes()

		scheduler := newScheduler(scrambler, evaluator)
		res := scheduler.Schedule(ctxt, &BlockInfo{}, slices.Values(txs), 100_000_000)
		require.NotEmpty(t, res)
	}
}
