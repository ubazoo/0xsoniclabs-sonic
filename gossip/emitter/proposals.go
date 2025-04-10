package emitter

import (
	"fmt"
	"math/big"

	"github.com/0xsoniclabs/sonic/eventcheck/epochcheck"
	"github.com/0xsoniclabs/sonic/evmcore"
	"github.com/0xsoniclabs/sonic/inter"
	"github.com/0xsoniclabs/sonic/opera"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/core/types"
)

const ProposalRetryInterval = 8 // number of frames between proposal attempts

func (em *Emitter) addProposal(
	event *inter.MutableEventPayload,
	sorted *transactionsByPriceAndNonce,
) error {

	const enableProposalDebugPrints = false

	lastSeenProposalNumber := idx.Block(0)
	lastSeenProposalAttempt := uint32(0)
	lastSeenProposalFrame := idx.Frame(0)
	//fmt.Printf("Adding proposals to event %v in frame %d\n", event.ID(), event.Frame())
	for _, parent := range event.Parents() {
		//fmt.Printf("\tParent %v\n", parent)
		payload := em.world.GetEventPayload(parent)
		envelope := payload.ProposalEnvelope()
		number := envelope.LastSeenProposalNumber
		attempt := envelope.LastSeenProposalAttempt
		frame := envelope.LastSeenProposalFrame
		//fmt.Printf("\t\tLast Proposal %d/%d @ frame=%d\n", number, attempt, frame)

		if number > lastSeenProposalNumber || number == envelope.LastSeenProposalNumber && attempt > lastSeenProposalAttempt {
			lastSeenProposalNumber = number
			lastSeenProposalAttempt = attempt
			lastSeenProposalFrame = frame
		}

	}

	// By default, we fill the envelope with the latest seen proposal information.
	// By adding this to all events, event validation can track the progress of
	// the block-proposing based on events and their parents. It also enables
	// the detection if invalid proposals.
	event.SetProposalEnvelope(&inter.ProposalEnvelope{
		LastSeenProposalNumber:  lastSeenProposalNumber,
		LastSeenProposalAttempt: lastSeenProposalAttempt,
		LastSeenProposalFrame:   lastSeenProposalFrame,
	})

	// TODO:
	// 1. Check that the emitter has the right to make a proposal
	// 2. Derive meta-information for proposal
	// 3. Create list of transactions for proposal

	// Get next expected block number.
	nextBlock := em.world.GetLatestBlockIndex() + 1

	// Get expected attempt for the next block.
	lastProposerFrame := em.GetFrameOfLastProposal()
	attempt := uint32(event.Frame()-lastProposerFrame) / ProposalRetryInterval

	// TODO: remove
	/*
		fmt.Printf(
			"validator=%d, frame=%d, nextBlock=%d, attempt=%d, lastEmittedBlock=%d, lastProposerFrame=%d\n",
			em.config.Validator.ID, event.Frame(), nextBlock, attempt, em.lastBlockProposedByThisEmitter, lastProposerFrame,
		)
	*/

	// If the next expected block was already proposed, skip the proposal.
	// TODO: this could be checked against the "last seen proposal" in the event
	// envelope instead of a member field.
	if nextBlock <= em.lastBlockProposedByThisEmitter {
		return nil
	}

	// Check whether this emitter is the proposer for the next block.
	proposer, err := inter.GetProposer(
		em.validators,
		nextBlock,
		attempt,
	)
	if err != nil || proposer != em.config.Validator.ID {
		return err
	}

	if enableProposalDebugPrints {
		fmt.Printf(
			"validator=%d, starting proposal for block %d/%d as part of frame %d (last proposal at frame %d)\n",
			em.config.Validator.ID, nextBlock, attempt, event.Frame(), lastProposerFrame,
		)
	}

	// Create the proposal for the next block.
	proposal := &inter.Proposal{
		Number:  nextBlock,
		Attempt: attempt,
		//ParentHash: em.world.GetBlockHash(nextBlock - 1),
		Time: event.CreationTime(),
		// PrevRandao: -- figure out what to use --
	}

	// TODO: add transactions to the proposal
	if err := em.addTransactionsToProposal(proposal, sorted); err != nil {
		return err
	}

	// TODO: remove
	if enableProposalDebugPrints {
		fmt.Printf(
			"validator=%d, completed proposal for block %d/%d with %d transactions\n",
			em.config.Validator.ID, nextBlock, attempt, len(proposal.Transactions),
		)
		for _, tx := range proposal.Transactions {
			fmt.Printf("\tTransaction with nonce %d\n", tx.Nonce())
		}
	}

	// Envelop and append the new proposal to the event.
	event.SetProposalEnvelope(&inter.ProposalEnvelope{
		LastSeenProposalNumber:  proposal.Number,
		LastSeenProposalAttempt: proposal.Attempt,
		LastSeenProposalFrame:   event.Frame(),
		Proposal:                proposal,
	})

	// Remember the new block as proposed.
	em.lastBlockProposedByThisEmitter = nextBlock
	return nil
}

func (em *Emitter) addTransactionsToProposal(
	proposal *inter.Proposal,
	sorted *transactionsByPriceAndNonce,
) error {

	// TODO:
	// - enforce gas limit
	// - enforce scrambled order of transactions

	// Create a block-processor instance to check that selected transactions
	// are indeed executable in the given order on the current state.

	block := &evmcore.EvmBlock{
		EvmHeader: evmcore.EvmHeader{
			Number:   new(big.Int).SetUint64(uint64(proposal.Number)),
			Time:     proposal.Time,
			GasLimit: 10_000_000_000, // TODO: get from network rules
			// TODO: add missing fields like base-fee, randao, ...
		},
	}

	// TODO: align this with c_block_callbacks.go

	rules := em.world.GetRules()
	chainCfg := rules.EvmChainConfig(em.world.GetUpgradeHeights())

	stateDb := em.world.StateDB()
	defer stateDb.Release()

	var usedGas uint64
	runTx := evmcore.NewStateProcessor(chainCfg, em.world).BeginBlock(
		block, stateDb, opera.DefaultVMConfig, &usedGas,
	)

	// sort transactions by price and nonce
	for tx, _ := sorted.Peek(); tx != nil; tx, _ = sorted.Peek() {
		resolvedTx := tx.Resolve()

		/*
			sender, _ := types.Sender(em.world.TxSigner, resolvedTx)
			nonce := stateDb.GetNonce(sender)
			fmt.Printf("Candidate transaction from sender %s, tx-nonce %d, db-nonce %d, ...\n", sender.Hex(), resolvedTx.Nonce(), nonce)
		*/

		// check transaction epoch rules (tx type, gas price)
		if epochcheck.CheckTxs(types.Transactions{resolvedTx}, rules) != nil {
			txsSkippedEpochRules.Inc(1)
			sorted.Pop()
			continue
		}

		// make sure the transaction can be processed
		_, skipped, err := runTx(len(proposal.Transactions), resolvedTx)
		if skipped || err != nil {
			if skipped {
				fmt.Printf("\tTransaction skipped\n")
			} else {
				fmt.Printf("\tExecution error: %v\n", err)
			}
			txsSkippedExecutionError.Inc(1)
			sorted.Pop()
			continue
		}

		// add transaction to the proposal
		proposal.Transactions = append(proposal.Transactions, resolvedTx)
		sorted.Shift()

		// TODO: check gas limit
	}
	return nil
}
