package scheduler

import (
	"context"
	"math/big"

	"github.com/0xsoniclabs/sonic/evmcore"
	"github.com/0xsoniclabs/sonic/inter/state"
	"github.com/0xsoniclabs/sonic/opera"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
)

//go:generate mockgen -source=evaluator.go -destination=evaluator_mock.go -package=scheduler

// evaluator is an internal interface for a component handling the evaluation of
// a transaction schedule.
type evaluator interface {
	// evaluate runs the evaluation of a transaction schedule. The provided
	// list of transactions is processed one-after-the-other until either the
	// gas limit is reached, the context is cancelled or all transactions are
	// processed.
	//
	// The resulting list of transactions contains all successfully processed
	// transactions in the order they were processed. The usedGas is the total
	// gas used by the transactions in the resulting list. The limitReached
	// boolean indicates whether the gas limit was reached during the evaluation
	// and some candidates were ignored due to insufficient gas.
	evaluate(
		context context.Context,
		blockInfo *BlockInfo,
		schedule []*types.Transaction,
		gasLimit uint64,
	) (
		processed []*types.Transaction,
		usedGas uint64,
		limitReached bool,
	)
}

// ----- Implementations -----

// executingEvaluator is an implementation of the evaluator interface that
// evaluates a transaction using a block-processor implementation.
type executingEvaluator struct {
	// factory is a factory for creating a fresh block-processing instance for
	// each individual evaluation run.
	factory processorFactory
}

// evaluate runs the evaluation of the provided transaction using a fresh
// instance of a transaction processor.
func (e *executingEvaluator) evaluate(
	context context.Context,
	blockInfo *BlockInfo,
	schedule []*types.Transaction,
	gasLimit uint64,
) (
	[]*types.Transaction,
	uint64,
	bool,
) {
	// Create a block-processor instance to check that selected transactions
	// are indeed executable in the given order on the current state.
	block := &evmcore.EvmBlock{
		EvmHeader: evmcore.EvmHeader{
			Number:      new(big.Int).SetUint64(uint64(blockInfo.Number)),
			Time:        blockInfo.Time,
			GasLimit:    blockInfo.GasLimit,
			Coinbase:    blockInfo.Coinbase,
			PrevRandao:  blockInfo.PrevRandao,
			BaseFee:     blockInfo.BaseFee.ToBig(),
			BlobBaseFee: blockInfo.BlobBaseFee.ToBig(),
		},
	}

	txProcessor := e.factory.beginBlock(block)
	defer txProcessor.release()

	reachedLimit := false
	remainingGas := gasLimit
	selected := make([]*types.Transaction, 0, len(schedule))
	for _, tx := range schedule {
		if context.Err() != nil {
			// context was cancelled, stop processing, return current results
			break
		}
		// test whether this transaction can be processed
		result, gasUsed := txProcessor.run(tx, remainingGas)
		switch result {
		case runSuccess:
			selected = append(selected, tx)
			remainingGas -= gasUsed
		case runSkipped:
			// Can be ignored, continue with next.
		case runOutOfGas:
			// Can be ignore as well, but remember that the gas limit was
			// reached. Since subsequent transactions may require less gas, the
			// remaining transactions need to be evaluated as well.
			reachedLimit = true
		}
	}
	return selected, gasLimit - remainingGas, reachedLimit
}

// ----- Internal Interfaces and Adapters -----

// processorFactory is an internal interface for a component that creates a
// transaction processor capable of test-running transactions in a block to
// be scheduled.
type processorFactory interface {
	beginBlock(*evmcore.EvmBlock) processor
}

// processor is an internal interface for a component that can process
// individual transactions to be scheduled in a block.
type processor interface {
	// run runs the given transaction in the context of the current block
	// and returns the result of the execution. The gas limit is the maximum
	// amount of gas that can be used by the transaction.
	run(
		tx *types.Transaction,
		gasLimit uint64,
	) (
		result runResult,
		gasUsed uint64,
	)

	// release releases the resources used by the processor. In particular, it
	// allows implementations to release temporary database state.
	release()
}

// runResult is an internal type used by the processor interface to report the
// success state of a transaction execution. The result is used to determine
// whether the transaction can be included in a block proposal or not. If not
// it allows to differentiate between a transaction that can not be executed
// (skipped) and a transaction that can not be executed because it does not fit
// into the block (out of gas).
type runResult int

const (
	runSuccess  runResult = iota // < successful execution
	runSkipped                   // < transaction can not be executed
	runOutOfGas                  // < transaction does not fit into the block
)

// ------ Adapters for the EVM ------

// evmProcessorFactory is an implementation of the processorFactory that wraps
// the EVM state processor implementation provided by the evmcore package.
type evmProcessorFactory struct {
	// chain provides access to the chain state retained by the client,
	// including the current chain configuration and the state database.
	chain Chain
}

type Chain interface {
	evmcore.DummyChain
	GetEvmChainConfig() *params.ChainConfig
	StateDB() state.StateDB
}

func (p *evmProcessorFactory) beginBlock(
	block *evmcore.EvmBlock,
) processor {
	// TODO: follow-up task - align this with c_block_callbacks.go
	chainCfg := p.chain.GetEvmChainConfig()
	vmConfig := opera.DefaultVMConfig
	state := p.chain.StateDB()

	stateProcessor := evmcore.NewStateProcessor(chainCfg, p.chain)
	return &evmProcessor{
		processor: stateProcessor.BeginBlock(block, state, vmConfig, nil),
		stateDb:   state,
	}
}

// evmProcessor is the implementation of the processor interface produced by the
// evmProcessorFactory. It retains an instance of the evmcore's
// TransactionProcessor, abstracted through the evmProcessorRunner interface, and
// the stateDb instance holding the temporary state of the EVM accumulating
// changes during the transaction execution. These changes are discarded when
// the processor is released.
type evmProcessor struct {
	processor evmProcessorRunner
	stateDb   state.StateDB
}

func (p *evmProcessor) run(
	tx *types.Transaction,
	gasLimit uint64,
) (
	result runResult,
	gasUsed uint64,
) {
	// Note: the index can be set to 0 since code running inside the EVM can not
	// obtain the position of a transaction in the block. It has thus no effect
	// on the scheduling of the transactions.
	receipt, skipped, err := p.processor.Run(0, tx)
	if skipped || err != nil {
		return runSkipped, 0
	}
	// TODO: forward the gas limit to the processor to avoid running unnecessary
	// computation steps in the EVM.
	if receipt.GasUsed > gasLimit {
		return runOutOfGas, 0
	}
	return runSuccess, receipt.GasUsed
}

func (p *evmProcessor) release() {
	p.stateDb.Release()
}

// evmProcessorRunner is an interface implemented by the evmcore's
// TransactionProcessor. The interface is defined instead of a direct dependency
// to avoid unnecessary dependencies and to facilitate mocking of the evmcore.
type evmProcessorRunner interface {
	Run(int, *types.Transaction) (*types.Receipt, bool, error)
}
