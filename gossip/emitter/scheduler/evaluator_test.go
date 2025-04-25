package scheduler

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/0xsoniclabs/sonic/evmcore"
	"github.com/0xsoniclabs/sonic/inter"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEvaluator_Evaluate_ForwardsMetaInfoToTheProcessor(t *testing.T) {
	ctrl := gomock.NewController(t)
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)

	number := idx.Block(2)
	time := inter.Timestamp(345)
	gasLimit := uint64(67)
	coinbase := common.Address{0, 8, 15}
	prevRandao := common.Hash{42, 73}
	baseFee := *uint256.NewInt(100)
	blobBaseFee := *uint256.NewInt(200)

	factory.EXPECT().beginBlock(&evmcore.EvmBlock{
		EvmHeader: evmcore.EvmHeader{
			Number:      big.NewInt(int64(number)),
			Time:        time,
			GasLimit:    gasLimit,
			Coinbase:    coinbase,
			PrevRandao:  prevRandao,
			BaseFee:     baseFee.ToBig(),
			BlobBaseFee: blobBaseFee.ToBig(),
		},
	}).Return(processor)
	processor.EXPECT().release()

	(&executingEvaluator{factory}).evaluate(
		t.Context(),
		&BlockInfo{
			Number:      idx.Block(number),
			Time:        time,
			GasLimit:    gasLimit,
			Coinbase:    coinbase,
			PrevRandao:  prevRandao,
			BaseFee:     baseFee,
			BlobBaseFee: blobBaseFee,
		},
		nil,
		0,
	)
}

func TestEvaluator_Evaluate_ExecutesGivenTransactionsAndCollectsDataOfSuccessfulOnes(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)

	tx1 := types.NewTx(&types.LegacyTx{Nonce: 1})
	tx2 := types.NewTx(&types.LegacyTx{Nonce: 2})
	tx3 := types.NewTx(&types.LegacyTx{Nonce: 3})
	txs := []*types.Transaction{tx1, tx2, tx3}

	const gasCosts = uint64(123_000)
	const gasLimit = 3*gasCosts + 1
	factory.EXPECT().beginBlock(gomock.Any()).Return(processor)
	processor.EXPECT().run(tx1, gasLimit).Return(runSuccess, gasCosts)
	processor.EXPECT().run(tx2, gasLimit-gasCosts).Return(runSuccess, gasCosts)
	processor.EXPECT().run(tx3, gasLimit-2*gasCosts).Return(runSuccess, gasCosts)
	processor.EXPECT().release()

	evaluator := &executingEvaluator{factory}
	order, usedGas, limitReached := evaluator.evaluate(
		t.Context(),
		&BlockInfo{},
		txs,
		3*gasCosts+1,
	)

	require.Equal(txs, order)
	require.Equal(3*gasCosts, usedGas)
	require.False(limitReached)
}

func TestEvaluator_Evaluate_IgnoresSkippedTransactions(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)

	tx1 := types.NewTx(&types.LegacyTx{Nonce: 1})
	tx2 := types.NewTx(&types.LegacyTx{Nonce: 2})
	tx3 := types.NewTx(&types.LegacyTx{Nonce: 3})
	txs := []*types.Transaction{tx1, tx2, tx3}

	const gasCosts = uint64(123_000)
	factory.EXPECT().beginBlock(gomock.Any()).Return(processor)
	processor.EXPECT().run(tx1, gomock.Any()).Return(runSuccess, gasCosts)
	processor.EXPECT().run(tx2, gomock.Any()).Return(runSkipped, uint64(0))
	processor.EXPECT().run(tx3, gomock.Any()).Return(runSuccess, gasCosts)
	processor.EXPECT().release()

	evaluator := &executingEvaluator{factory}
	order, usedGas, limitReached := evaluator.evaluate(
		t.Context(),
		&BlockInfo{},
		txs,
		3*gasCosts+1,
	)

	require.Equal([]*types.Transaction{tx1, tx3}, order)
	require.Equal(2*gasCosts, usedGas)
	require.False(limitReached)
}

func TestEvaluator_Evaluate_RespectsTransactionsRunningOutOfGas(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)

	tx1 := types.NewTx(&types.LegacyTx{Nonce: 1})
	tx2 := types.NewTx(&types.LegacyTx{Nonce: 2})
	tx3 := types.NewTx(&types.LegacyTx{Nonce: 3})
	txs := []*types.Transaction{tx1, tx2, tx3}

	const gasCosts = uint64(123_000)
	const gasLimit = gasCosts + 1
	factory.EXPECT().beginBlock(gomock.Any()).Return(processor)
	processor.EXPECT().run(tx1, gasLimit).Return(runSuccess, gasCosts)
	processor.EXPECT().run(tx2, gasLimit-gasCosts).Return(runOutOfGas, uint64(0))
	processor.EXPECT().run(tx3, gasLimit-gasCosts).Return(runOutOfGas, uint64(0))
	processor.EXPECT().release()

	evaluator := &executingEvaluator{factory}
	order, usedGas, limitReached := evaluator.evaluate(
		t.Context(),
		&BlockInfo{},
		txs,
		gasLimit,
	)

	require.Equal([]*types.Transaction{tx1}, order)
	require.Equal(gasCosts, usedGas)
	require.True(limitReached)
}

func TestEvaluator_Evaluate_CanBeCanceled(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	factory := NewMockprocessorFactory(ctrl)
	processor := NewMockprocessor(ctrl)

	tx1 := types.NewTx(&types.LegacyTx{Nonce: 1})
	tx2 := types.NewTx(&types.LegacyTx{Nonce: 2})
	tx3 := types.NewTx(&types.LegacyTx{Nonce: 3})
	txs := []*types.Transaction{tx1, tx2, tx3}

	const gasCosts = uint64(123_000)
	context, cancel := context.WithCancel(t.Context())
	factory.EXPECT().beginBlock(gomock.Any()).Return(processor)

	// Cancel the execution after the second transaction should prevent the
	// third from being executed.
	processor.EXPECT().run(tx1, gomock.Any()).Return(runSuccess, gasCosts)
	processor.EXPECT().run(tx2, gomock.Any()).DoAndReturn(
		func(tx *types.Transaction, gas uint64) (runResult, uint64) {
			cancel()
			return runSuccess, gasCosts
		},
	)
	processor.EXPECT().release()

	evaluator := &executingEvaluator{factory}
	order, usedGas, limitReached := evaluator.evaluate(
		context,
		&BlockInfo{},
		txs,
		10*gasCosts,
	)

	require.Equal([]*types.Transaction{tx1, tx2}, order)
	require.Equal(2*gasCosts, usedGas)
	require.False(limitReached)
}

func TestEvmProcessor_Run_IfExecutionSucceeds_ReportsSuccessAndGasUsage(t *testing.T) {
	ctrl := gomock.NewController(t)
	runner := NewMockevmProcessorRunner(ctrl)

	runner.EXPECT().Run(0, nil).Return(&types.Receipt{
		GasUsed: 10,
	}, false, nil)

	processor := &evmProcessor{processor: runner}
	result, gasUsed := processor.run(nil, 50)
	require.Equal(t, runSuccess, result)
	require.Equal(t, uint64(10), gasUsed)
}

func TestEvmProcessor_Run_IfGasLimitIsExceeded_ReportsOutOfGas(t *testing.T) {
	ctrl := gomock.NewController(t)
	runner := NewMockevmProcessorRunner(ctrl)

	runner.EXPECT().Run(0, nil).Return(&types.Receipt{
		GasUsed: 100,
	}, false, nil)

	processor := &evmProcessor{processor: runner}
	result, _ := processor.run(nil, 50)
	require.Equal(t, runOutOfGas, result)
}

func TestEvmProcessor_Run_IfExecutionFailed_ReportsSkipped(t *testing.T) {

	t.Run("skipped", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		runner := NewMockevmProcessorRunner(ctrl)
		runner.EXPECT().Run(0, nil).Return(nil, true, nil)
		processor := &evmProcessor{processor: runner}
		result, _ := processor.run(nil, 50)
		require.Equal(t, runSkipped, result)
	})

	t.Run("failed", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		runner := NewMockevmProcessorRunner(ctrl)
		runner.EXPECT().Run(0, nil).Return(nil, false, fmt.Errorf("failed"))
		processor := &evmProcessor{processor: runner}
		result, _ := processor.run(nil, 50)
		require.Equal(t, runSkipped, result)
	})
}
