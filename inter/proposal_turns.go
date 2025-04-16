package inter

import "github.com/Fantom-foundation/lachesis-base/inter/idx"

const TurnTimeoutInFrames = 8 // number of frames after which a turn is considered failed

type Turn uint32

type ProposalSummary struct {
	Turn  Turn
	Block idx.Block
	Frame idx.Frame
}

// IsValidTurnProgression determines whether a turn
func IsValidTurnProgression(
	previous ProposalSummary,
	next ProposalSummary,
) bool {
	// TODO: proof these conditions;

	// In the good case, the subsequent proposal is for the succeeding turn
	// for the succeeding block in a subsequent frame. This does not require
	// a minimum waiting period.
	if previous.Turn+1 == next.Turn {
		return previous.Block+1 == next.Block && previous.Frame < next.Frame
	}

	// If there is a failed turn (either not proposed or not accepted), the
	// next turn must be at least ProposalTimeoutInterval frames after the
	// previous turn. This is to give a proposer enough time to propose
	// a new block before being declared failed.
	if next.Turn < previous.Turn {
		return false
	}

	gap := idx.Frame(TurnTimeoutInFrames * (next.Turn - previous.Turn))
	return previous.Frame+gap < next.Frame
}
