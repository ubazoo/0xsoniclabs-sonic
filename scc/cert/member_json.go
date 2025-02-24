package cert

import (
	"fmt"

	"github.com/0xsoniclabs/sonic/scc"
	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/0xsoniclabs/sonic/scc/cert/serialization"
)

// MemberJson is a JSON friendly representation of a scc.Member.
type MemberJson struct {
	PublicKey         serialization.HexBytes48 `json:"publicKey" gencodec:"required"`
	ProofOfPossession serialization.HexBytes96 `json:"proofOfPossession" gencodec:"required"`
	Weight            uint64                   `json:"weight" gencodec:"required"`
}

// String returns the JSON string representation of the MemberJson.
func (m MemberJson) String() string {
	return fmt.Sprintf(`{"publicKey":"%v","proofOfPossession":"%v","weight":%d}`,
		m.PublicKey, m.ProofOfPossession, m.Weight)
}

// ToMember converts the MemberJson to a scc.Member.
func (m MemberJson) ToMember() (scc.Member, error) {
	publicKey, err := bls.DeserializePublicKey(m.PublicKey)
	if err != nil {
		return scc.Member{}, err
	}
	proofOfPossession, err := bls.DeserializeSignature(m.ProofOfPossession)
	if err != nil {
		return scc.Member{}, err
	}
	return scc.Member{
		PublicKey:         publicKey,
		ProofOfPossession: proofOfPossession,
		VotingPower:       m.Weight,
	}, nil
}

// MemberToJson converts a scc.Member to a MemberJson.
func MemberToJson(m scc.Member) MemberJson {
	return MemberJson{
		PublicKey:         serialization.HexBytes48(m.PublicKey.Serialize()),
		ProofOfPossession: serialization.HexBytes96(m.ProofOfPossession.Serialize()),
		Weight:            m.VotingPower,
	}
}
