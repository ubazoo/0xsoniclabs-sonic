package scheduler

import (
	"context"
	"iter"
	"math/big"

	"github.com/0xsoniclabs/sonic/inter"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

// Scheduler implements a scheduling algorithm for transactions facilitating
// the selection of transactions for inclusion in a block. The scheduler thereby
// solves the dynamic scheduling problem defined by the pending transactions in
// the transaction pool constraint by the current chain state.
type Scheduler struct {
	scrambler scrambler
	evaluator evaluator
}

// NewScheduler creates a new scheduler to be used in the the block emitter. The
// provided world interface is used to obtain the current state of the chain
// whenever transactions need to be scheduled.
func NewScheduler(chain Chain) *Scheduler {
	return newScheduler(
		prototypeScrambler{},
		&executingEvaluator{factory: &evmProcessorFactory{chain: chain}},
	)
}

// newScheduler is an internal factory with customizable scrambler and evaluator
// implementations. Its flexibility is mainly intended for testing purposes.
func newScheduler(
	scrambler scrambler,
	evaluator evaluator,
) *Scheduler {
	return &Scheduler{
		scrambler: scrambler,
		evaluator: evaluator,
	}
}

// Schedule attempts to identify a subset of executable transactions from the
// given list of candidates provided through the sortedTxs iterator. The order
// of the provided transactions is considered to be the intended priority.
//
// The scheduler attempts to maximize the following property:
//
//	gasUsage(scrambled(candidates)) <= gasLimit
//
// where candidates is a prefix of sortedTxs.
//
// The provided list of sortedTxs is expected to only enumerate transactions
// with unique sender/nonce pairs. If there are duplicates, the scheduler is
// free to schedule them in any order, making at most one of them part of the
// resulting schedule.
//
// The resulting execution order is then the scrambled(candidates) list. This
// should have the following effect:
//   - transactions with higher priority are favored for inclusion in the block
//   - the order of included transactions is not influenced by the priority
//
// The scheduler can be stopped at any time by cancelling the provided context.
// If cancelled, the best known solution found so far is returned.
func (s *Scheduler) Schedule(
	context context.Context,
	blockInfo *BlockInfo,
	sortedTxs iter.Seq[*types.Transaction],
	gasLimit uint64,
) []*types.Transaction {

	// Search for a good schedule using a binary search.
	var bestOrder []*types.Transaction
	var bestGasUsed uint64
	var bestCandidateSize int

	// Eval is a utility function that evaluates a given candidate list, tracking
	// the best transaction order seen over several invocations.
	eval := func(candidates []*types.Transaction) bool {
		// Scramble and evaluate the resulting list of transactions. Transactions
		// are verifiably scrambled to limit the influence of proposers on the
		// transaction order.
		signer := types.LatestSignerForChainID(blockInfo.ChainID)
		randao := blockInfo.PrevRandao.Big().Uint64() // TODO: convert to uint64 in a deterministic way
		scrambled := s.scrambler.scramble(candidates, signer, randao)
		order, gasUsed, reachedLimit := s.evaluator.evaluate(
			context, blockInfo, scrambled, gasLimit,
		)
		// The ideal solution we are looking for maximizes the gas usage while
		// considering the shortest possible candidate list.
		isBetter := gasUsed > bestGasUsed ||
			(gasUsed == bestGasUsed && len(candidates) < bestCandidateSize)

		if isBetter {
			bestGasUsed = gasUsed
			bestOrder = order
			bestCandidateSize = len(candidates)
		}
		return reachedLimit
	}

	// Step 1: find an upper bound for the number of transactions to consider.
	candidates := []*types.Transaction{}
	next, stop := iter.Pull(sortedTxs)
	defer stop()

	reachedGasLimit := false
	for i := 1; ; i *= 2 {
		if context.Err() != nil {
			break
		}

		// Expand the number of candidates.
		hasGrown := false
		for len(candidates) < i {
			tx, ok := next()
			if !ok {
				break
			}
			candidates = append(candidates, tx)
			hasGrown = true
		}
		if !hasGrown {
			break
		}

		// Evaluate the current candidates and stop if the gas limit was reached.
		if reachedLimit := eval(candidates); reachedLimit {
			reachedGasLimit = true
			break
		}
	}

	// Step 2: fine-tune if not all transactions were used.
	if reachedGasLimit {
		// Binary search the interval between the size of the best known
		// solution and the size of the candidate list causing the gas limit
		// to be exceeded.
		low := len(bestOrder)
		high := len(candidates)
		for low < high {
			if context.Err() != nil {
				break
			}
			mid := (low + high) / 2
			if reachedLimit := eval(candidates[:mid]); reachedLimit {
				high = mid
			} else {
				low = mid + 1
			}
		}
	}

	return bestOrder
}

// BlockInfo contains all the block meta-information accessible within EVM
// code executions. These parameters are required to produce reliable results
// of transaction executions during the scheduling. They should thus be aligned
// with the parameters used once the block is confirmed and executed on the
// chain.
type BlockInfo struct {
	// Note: ChainID would be another candidate field to be included, but it is
	// not block specific, and thus not part of the block header to be configured
	// by the scheduler for try-running transactions.

	ChainID *big.Int

	// Number of the block being scheduled, accessible by the NUMBER opcode.
	Number idx.Block

	// Time is the block time of the block being scheduled, accessible by the
	// TIMESTAMP opcode.
	Time inter.Timestamp

	// GasLimit for the full block, as visible within the EVM. This is not
	// aligned with the actual gas limit available for being scheduled in
	// a block since overheads for epoch sealing and other transactions need
	// to be accounted for. In practice, this is a constant set by the network
	// rules orders of magnitude larger than any realistic block limit.
	// This is accessible by the GASLIMIT opcode.
	GasLimit uint64

	// Coinbase, as seen by the COINBASE opcode.
	Coinbase common.Address

	// PrevRandao, as seen by the PREVRANDAO opcode.
	PrevRandao common.Hash

	// BaseFee, as seen by the BASEFEE opcode.
	BaseFee uint256.Int

	// BlobBaseFee, as seen by the BLOBBASEFEE opcode.
	BlobBaseFee uint256.Int
}
