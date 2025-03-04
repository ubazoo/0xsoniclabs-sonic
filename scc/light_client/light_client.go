package light_client

import (
	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/cert"
)

// LightClientState holds the state of a light client.
// It contains the latest committee, period and block head the client has validated.
type LightClientState struct {
	committee scc.Committee
	period    scc.Period

	head cert.BlockStatement
}

// NewLightClient creates a new light client state with the given committee.
func NewLightClient(committee scc.Committee) *LightClientState {
	return &LightClientState{committee: committee}
}

// Head returns the block statement of what the light client considers the head of the chain.
func (lc *LightClientState) Head() cert.BlockStatement {
	return lc.head
}

// Period returns the period up to which the light client has verified the chain.
func (lc *LightClientState) Period() scc.Period {
	return lc.period
}
