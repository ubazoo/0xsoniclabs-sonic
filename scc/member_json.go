package scc

import (
	"fmt"

	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/utils/jsonhex"
)

// MemberJson is a JSON friendly representation of a scc.Member.
type MemberJson struct {
	PublicKey         jsonhex.Bytes48 `json:"publicKey"`
	ProofOfPossession jsonhex.Bytes96 `json:"proofOfPossession"`
	Weight            uint64          `json:"weight"`
}

// String returns the JSON string representation of the MemberJson.
func (m MemberJson) String() string {
	return fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		m.PublicKey, m.ProofOfPossession, m.Weight)
}

// ToMember converts the MemberJson to a scc.Member.
func (m MemberJson) ToMember() (Member, error) {
	publicKey, err := bls.DeserializePublicKey(m.PublicKey)
	if err != nil {
		return Member{}, err
	}
	proofOfPossession, err := bls.DeserializeSignature(m.ProofOfPossession)
	if err != nil {
		return Member{}, err
	}
	return Member{
		PublicKey:         publicKey,
		ProofOfPossession: proofOfPossession,
		VotingPower:       m.Weight,
	}, nil
}

// MemberToJson converts a scc.Member to a MemberJson.
func MemberToJson(m Member) MemberJson {
	return MemberJson{
		PublicKey:         jsonhex.Bytes48(m.PublicKey.Serialize()),
		ProofOfPossession: jsonhex.Bytes96(m.ProofOfPossession.Serialize()),
		Weight:            m.VotingPower,
	}
}
