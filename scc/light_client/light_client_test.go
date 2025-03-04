package light_client

import (
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/stretchr/testify/require"
)

func TestLightClient_CanInitializeFromCommittee(t *testing.T) {
	require := require.New(t)

	key1 := bls.NewPrivateKey()
	mem1 := memberFromKey(key1)
	committee := scc.NewCommittee(mem1)
	lc := NewLightClient(committee)

	require.Equal(committee, lc.committee)

}

func memberFromKey(key bls.PrivateKey) scc.Member {
	return scc.Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       1,
	}
}
