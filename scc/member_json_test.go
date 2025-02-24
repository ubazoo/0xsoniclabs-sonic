package scc

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/utils/jsonhex"
	"github.com/stretchr/testify/require"
)

func TestMemberJson_String(t *testing.T) {
	m := MemberJson{}
	expected := fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		jsonhex.Bytes48{}, jsonhex.Bytes96{}, uint64(0))
	require.Equal(t, expected, m.String())
}

func TestMemberJson_ToMember_InvalidPublicKey(t *testing.T) {
	m := MemberJson{
		PublicKey:         jsonhex.Bytes48{},
		ProofOfPossession: jsonhex.Bytes96(bls.Signature{}.Serialize()),
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.Error(t, err)
}

func TestMemberJson_ToMember_InvalidProofOfPossession(t *testing.T) {
	m := MemberJson{
		PublicKey:         jsonhex.Bytes48(bls.NewPrivateKey().PublicKey().Serialize()),
		ProofOfPossession: jsonhex.Bytes96{},
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.Error(t, err)
}

func TestMemberJson_ToMember_Valid(t *testing.T) {
	key := bls.NewPrivateKey()
	m := MemberJson{
		PublicKey:         jsonhex.Bytes48(key.PublicKey().Serialize()),
		ProofOfPossession: jsonhex.Bytes96(key.Sign([]byte{}).Serialize()),
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.NoError(t, err)
}

func TestMemberJson_EndToEnd(t *testing.T) {
	key := bls.NewPrivateKey()
	m := Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       uint64(0),
	}

	json := MemberToJson(m)
	str := json.String()
	wantString := fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		jsonhex.Bytes48(key.PublicKey().Serialize()),
		jsonhex.Bytes96(key.GetProofOfPossession().Serialize()),
		uint64(0))
	require.Equal(t, wantString, str)

	m2, err := json.ToMember()
	require.NoError(t, err)
	require.Equal(t, m, m2)
}
