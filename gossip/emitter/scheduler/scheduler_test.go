package scheduler

import (
	"context"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

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
		func([]*types.Transaction) []*types.Transaction {
			counter++
			return []*types.Transaction{
				types.NewTx(&types.LegacyTx{Nonce: counter}),
			}
		},
	).Times(N)

	evaluator.EXPECT().evaluate(any, any, any, any).DoAndReturn(
		func(
			_ context.Context, _ *BlockInfo, txs []*types.Transaction, _ uint64,
		) ([]*types.Transaction, uint64, bool) {
			t.Helper()
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
		func(
			_ context.Context, _ *BlockInfo, txs []*types.Transaction, _ uint64,
		) ([]*types.Transaction, uint64, bool) {
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

	// Simulate subsets of transactions that can be successfully executed.
	tests := map[string]func(*types.Transaction) bool{
		"all pass": func(tx *types.Transaction) bool {
			return true
		},
		"all fail": func(tx *types.Transaction) bool {
			return false
		},
		"even only": func(tx *types.Transaction) bool {
			return tx.Nonce()%2 == 0
		},
		"prime only": func(tx *types.Transaction) bool {
			n := int(tx.Nonce())
			for i := 2; i*i <= n; i++ {
				if n%i == 0 {
					return false
				}
			}
			return n > 1
		},
	}

	for name, runnable := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			scrambler := &prototypeScrambler{}

			// Create an evaluator that accepts selected transactions and each
			// of the passing transactions consumes a fixed amount of gas.
			const perTxGasCosts = 2
			factory := NewMockprocessorFactory(ctrl)
			processor := NewMockprocessor(ctrl)
			factory.EXPECT().beginBlock(gomock.Any()).Return(processor).AnyTimes()
			processor.EXPECT().release().AnyTimes()
			processor.EXPECT().run(gomock.Any(), gomock.Any()).DoAndReturn(
				func(tx *types.Transaction, remainingGas uint64) (runResult, uint64) {
					if !runnable(tx) {
						return runSkipped, 0
					}
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
			valid := []*types.Transaction{}
			for i := range uint64(N) {
				tx := types.NewTx(&types.LegacyTx{Nonce: i})
				txs = append(txs, tx)
				if runnable(tx) {
					valid = append(valid, tx)
				}
			}

			// Test that for any gas limit the runnable transactions with the
			// highest priority are selected.
			// Note: priority is defined by the input order.
			scheduler := newScheduler(scrambler, evaluator)
			for limit := range perTxGasCosts*N + 2 {
				result := scheduler.Schedule(
					t.Context(),
					&BlockInfo{},
					slices.Values(txs),
					uint64(limit),
				)

				want := valid[:min(limit/perTxGasCosts, len(valid))]
				require.ElementsMatch(t, want, result)
			}
		})
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
				t.Helper()
				require.LessOrEqual(t, counter, numEvaluationsBeforeCancel)
				if counter == numEvaluationsBeforeCancel {
					cancel()
				}
				counter++

				// To reach both phases of the scheduling, the exponential growth
				// as well as the binary search, the gas limit needs to be reached.
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
