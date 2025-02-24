package cert

import (
	"fmt"
	"testing"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert/serialization"
	"github.com/stretchr/testify/require"
)

func TestMemberJson_String(t *testing.T) {
	m := MemberJson{}
	expected := fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		serialization.HexBytes48{}, serialization.HexBytes96{}, uint64(0))
	require.Equal(t, expected, m.String())
}

func TestMemberJson_ToMember_InvalidPublicKey(t *testing.T) {
	m := MemberJson{
		PublicKey:         serialization.HexBytes48{},
		ProofOfPossession: serialization.HexBytes96(bls.Signature{}.Serialize()),
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.Error(t, err)
}

func TestMemberJson_ToMember_InvalidProofOfPossession(t *testing.T) {
	m := MemberJson{
		PublicKey:         serialization.HexBytes48(bls.NewPrivateKey().PublicKey().Serialize()),
		ProofOfPossession: serialization.HexBytes96{},
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.Error(t, err)
}

func TestMemberJson_ToMember_Valid(t *testing.T) {
	key := bls.NewPrivateKey()
	m := MemberJson{
		PublicKey:         serialization.HexBytes48(key.PublicKey().Serialize()),
		ProofOfPossession: serialization.HexBytes96(key.Sign([]byte{}).Serialize()),
		Weight:            uint64(0),
	}
	_, err := m.ToMember()
	require.NoError(t, err)
}

func TestMemberJson_EndToEnd(t *testing.T) {
	key := bls.NewPrivateKey()
	m := scc.Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       uint64(0),
	}

	json := MemberToJson(m)
	str := json.String()
	wantString := fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		serialization.HexBytes48(key.PublicKey().Serialize()),
		serialization.HexBytes96(key.GetProofOfPossession().Serialize()),
		uint64(0))
	require.Equal(t, wantString, str)

	m2, err := json.ToMember()
	require.NoError(t, err)
	require.Equal(t, m, m2)
}
