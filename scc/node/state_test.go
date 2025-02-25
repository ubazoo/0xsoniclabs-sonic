package node

import (
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/stretchr/testify/require"
)

func TestInMemoryState_RetainsCurrentState(t *testing.T) {
	committee := scc.NewCommittee(scc.Member{})
	state := newInMemoryState(committee)
	require.Equal(t, committee, state.GetCurrentCommittee())
}
