package proposalcheck

import (
	"errors"

	"github.com/0xsoniclabs/sonic/inter"
	"github.com/0xsoniclabs/sonic/inter/validatorpk"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
)

// TODO: document and make const
var (
	ErrMissingProposalEnvelope = errors.New("event has no proposal envelope")
	ErrInvalidProposer         = errors.New("event contains proposal of invalid proposer")
	ErrInvalidProposalTime     = errors.New("event contains proposal with invalid time")

	ErrInvalidLastSeenTurnNumber    = errors.New("event contains invalid last seen turn number")
	ErrInvalidLastSeenProposedBlock = errors.New("event contains invalid last seen proposed block")
	ErrInvalidLastSeenProposalFrame = errors.New("event contains invalid last seen proposal frame")

	ErrInvalidProposalNumber = errors.New("event contains invalid proposal number")

	ErrVersion3MustNotContainTransactions      = errors.New("event with version 3 must not contain transactions")
	ErrVersion3MustNotContainBlockVotes        = errors.New("event with version 3 must not contain block votes")
	ErrVersion3MustNotContainEpochVotes        = errors.New("event with version 3 must not contain epoch votes")
	ErrVersion3MustNotContainMisbehaviorProofs = errors.New("event with version 3 must not contain misbehavior proofs")
)

// Checker that block proposal and envelope information in events is valid.
type Checker struct {
	reader Reader
}

type Reader interface {
	GetEpochValidators() *pos.Validators                                // < validate proposer
	GetEpochBlockStart(idx.Epoch) idx.Block                             // < validate genesis block seen
	GetEpochPubKeysOf(idx.Epoch) map[idx.ValidatorID]validatorpk.PubKey // < validate randao value
	GetEventPayload(hash.Event) inter.EventPayloadI                     // < validate seen proposals
}

// New creates a new Checker validating the content of an event's proposal
// envelope.
func New(reader Reader) *Checker {
	return &Checker{
		reader: reader,
	}
}

// Validate event
func (v *Checker) Validate(e inter.EventPayloadI) error {

	// TODO: check the following properties
	// - last-seen fields are correctly filled
	// - a new proposal is not older than the last seen proposal
	// - the proposer has the right to propose
	// - if it is a re-proposal, sufficient time has passed
	// - the proposed block is valid, thus
	//   - the block number is correct
	//   - the attempt number is correct
	//   - the base-fee is correct
	//   - all transactions can be executed
	//   - the total gas used is within the limit of the accepted network throughput

	// This check only concerns events of protocol version 3.
	if e.Version() != 3 {
		return nil // all fine with other events
	}

	// None of the version 1 or 2 payload fields must be present.
	if e.AnyTxs() {
		return ErrVersion3MustNotContainTransactions
	}
	if e.AnyBlockVotes() {
		return ErrVersion3MustNotContainBlockVotes
	}
	if e.AnyEpochVote() {
		return ErrVersion3MustNotContainEpochVotes
	}
	if e.AnyMisbehaviourProofs() {
		return ErrVersion3MustNotContainMisbehaviorProofs
	}

	// -- Envelope Checks --

	// Check that there is an envelope.
	envelope := e.ProposalEnvelope()
	if envelope == nil {
		return ErrMissingProposalEnvelope
	}

	// Check that meta information was successfully propagated.
	wantLastSeenProposalTurn := inter.Turn(0)
	wantLastSeenProposedBlock := idx.Block(0)
	wantLastSeenProposalFrame := idx.Frame(0)
	if parents := e.Parents(); len(parents) == 0 {
		wantLastSeenProposedBlock = v.reader.GetEpochBlockStart(e.Epoch())
	} else {
		for _, parent := range e.Parents() {
			// TODO: make sure the parent event is always present before running this test!!
			payload := v.reader.GetEventPayload(parent)
			if payload == nil {
				return errors.New("missing parent payload")
			}
			if payload.Version() != 3 {
				return errors.New("parent event version mismatch")
			}
			envelope := payload.ProposalEnvelope()
			if envelope.LastSeenProposalTurn > wantLastSeenProposalTurn {
				wantLastSeenProposalTurn = envelope.LastSeenProposalTurn
			}
			if envelope.LastSeenProposedBlock > wantLastSeenProposedBlock {
				wantLastSeenProposedBlock = envelope.LastSeenProposedBlock
			}
			if envelope.LastSeenProposalFrame > wantLastSeenProposalFrame {
				wantLastSeenProposalFrame = envelope.LastSeenProposalFrame
			}
		}
	}

	// If a proposal is present, check that it is the next expected proposal.
	proposal := envelope.Proposal
	if proposal != nil {
		newTurn := envelope.LastSeenProposalTurn
		if !inter.IsValidTurnProgression(
			inter.ProposalSummary{
				Turn:  wantLastSeenProposalTurn,
				Block: wantLastSeenProposedBlock,
				Frame: wantLastSeenProposalFrame,
			},
			inter.ProposalSummary{
				Turn:  newTurn,
				Block: proposal.Number,
				Frame: e.Frame(),
			},
		) {
			return ErrInvalidProposalNumber
		}

		// If there is a proposal, it is the last seen proposal.
		wantLastSeenProposalTurn = newTurn
		wantLastSeenProposedBlock = proposal.Number
		wantLastSeenProposalFrame = e.Frame()
	}

	// Check that the last seen proposal information is correct.
	if envelope.LastSeenProposalTurn != wantLastSeenProposalTurn {
		return ErrInvalidLastSeenTurnNumber
	}
	if envelope.LastSeenProposedBlock != wantLastSeenProposedBlock {
		return ErrInvalidLastSeenProposedBlock
	}
	if envelope.LastSeenProposalFrame != wantLastSeenProposalFrame {
		return ErrInvalidLastSeenProposalFrame
	}

	// -- Proposal Checks --

	if proposal == nil {
		return nil
	}

	// Check that the creator of the event is allowed to make the present proposal.
	proposer, err := inter.GetProposer(
		v.reader.GetEpochValidators(),
		envelope.LastSeenProposalTurn,
	)
	if err != nil {
		return err
	}
	if proposer != e.Creator() {
		return ErrInvalidProposer
	}

	// Check that the proposed block time is equal to the median time.
	if proposal.Time != e.MedianTime() {
		return ErrInvalidProposalTime
	}

	// For these checks access to the preceding block is required:
	// - the right hash of the preceding block
	// - the use of the correct base-fee price
	// - the use of the correct randao value

	// For these checks access to the preceding block's state is required:
	// - all transactions are processable
	// - the total gas used is within the limit of the accepted network throughput

	// all fine
	return nil
}
