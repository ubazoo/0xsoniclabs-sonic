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
	// candidates are processed one-after-the-other until the gas limit is
	// reached, the context is cancelled or all candidates are processed.
	//
	// The resulting list of transactions contains all successfully processed
	// transactions in the order they were processed. The gas used is the total
	// gas used by the transactions in the resulting list. The limitReached
	// boolean indicates whether the gas limit was reached during the evaluation
	// and some candidates were ignored.
	evaluate(
		context context.Context,
		blockInfo *BlockInfo,
		candidates []*types.Transaction,
		gasLimit uint64,
	) (
		processed []*types.Transaction,
		usedGas uint64,
		limitReached bool,
	)
}

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
	transactions []*types.Transaction,
	gasLimit uint64,
) (
	[]*types.Transaction, // list of selected transactions in execution order
	uint64, // gas used by produced execution order
	bool, // true if the gas limit was reached and candidates were ignored
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

	txProcessor := e.factory.BeginBlock(block)
	defer txProcessor.Release()

	// Run transaction scrambler to make execution order verifiable random.
	reachedLimit := false
	remainingGas := gasLimit
	selected := make([]*types.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		if context.Err() != nil {
			// context was cancelled, stop processing, return current results
			break
		}
		// test whether this transaction can be processed
		result, gasUsed := txProcessor.Run(tx, remainingGas)
		switch result {
		case runSuccess:
			// add transaction to the resulting schedule
			selected = append(selected, tx)
			remainingGas -= gasUsed
		case runSkipped:
		// continue with next
		case runOutOfGas:
			// ignore as well, but remember that the gas limit was reached
			reachedLimit = true
		}
	}
	return selected, gasLimit - remainingGas, reachedLimit
}

// ----- Interfaces and Adapters -----

// processorFactory is an internal interface for a component that creates a
// transaction processor capable of test-running transactions in a block to
// be scheduled.
type processorFactory interface {
	BeginBlock(*evmcore.EvmBlock) processor
}

// processor is an internal interface for a component that can process
// individual transactions to be scheduled in a block.
type processor interface {
	// Run runs the given transaction in the context of the current block
	// and returns the result of the execution. The gas limit is the maximum
	// amount of gas that can be used by the transaction.
	Run(
		tx *types.Transaction,
		remainingGas uint64,
	) (
		result runResult,
		gasUsed uint64,
	)

	// Release releases the resources used by the processor. In particular, it
	// allows implementations to release temporary database state.
	Release()
}

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
	// world provides access to the client state, including the current chain
	// configuration and the state database.
	world External
}

type External interface {
	evmcore.DummyChain
	GetEvmChainConfig() *params.ChainConfig
	StateDB() state.StateDB
}

func (p *evmProcessorFactory) BeginBlock(
	block *evmcore.EvmBlock,
) processor {
	// TODO: align this with c_block_callbacks.go
	chainCfg := p.world.GetEvmChainConfig()
	vmConfig := opera.DefaultVMConfig
	state := p.world.StateDB()

	stateProcessor := evmcore.NewStateProcessor(chainCfg, p.world)
	return &evmProcessor{
		processor: stateProcessor.BeginBlock(block, state, vmConfig, nil),
		stateDb:   state,
	}
}

type evmProcessor struct {
	processor evmProcessorRunner
	stateDb   state.StateDB
}

func (p *evmProcessor) Run(
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

func (p *evmProcessor) Release() {
	p.stateDb.Release()
}

type evmProcessorRunner interface {
	Run(int, *types.Transaction) (*types.Receipt, bool, error)
}
