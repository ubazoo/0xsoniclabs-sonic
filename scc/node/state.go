package node

import "github.com/0xsoniclabs/sonic/scc"

//go:generate mockgen -source=state.go -destination=state_mock.go -package=node

// State is an interface for the current state of an SCC node. The state's main
// responsibility is to track the composition of the current committee.
type State interface {
	// GetCurrentCommittee returns a snapshot of the current committee.
	GetCurrentCommittee() scc.Committee

	// TODO: add committee mutation support
}

// inMemoryState is an in-memory implementation of the State interface. It
// retains all state information in memory and does not persist it.
type inMemoryState struct {
	committee scc.Committee

	// TODO: update internal structure to support committee mutation
}

func newInMemoryState(committee scc.Committee) State {
	return &inMemoryState{
		committee: committee,
	}
}

func (s *inMemoryState) GetCurrentCommittee() scc.Committee {
	return s.committee
}
