package scc

import (
	"fmt"

	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/holiman/uint256"
)

// Member is a member of a committee. Members are identified by their public key.
// To defend against rogue key attacks, members must provide a proof of possession
// for their public key. The voting power of a member determines their relative
// influence in committees.
type Member struct {
	PublicKey         bls.PublicKey
	ProofOfPossession bls.Signature
	VotingPower       uint256.Int
}

func (m Member) Validate() error {
	if !m.PublicKey.Validate() {
		return fmt.Errorf("invalid public key")
	}
	if !m.PublicKey.CheckProofOfPossession(m.ProofOfPossession) {
		return fmt.Errorf("invalid proof of possession")
	}
	if m.VotingPower.IsZero() {
		return fmt.Errorf("invalid zero voting power")
	}
	return nil
}

func (m Member) Serialize() [48 + 96 + 32]byte {
	res := [48 + 96 + 32]byte{}
	*(*[48]byte)(res[0:]) = m.PublicKey.Serialize()
	*(*[96]byte)(res[48:]) = m.ProofOfPossession.Serialize()
	*(*[32]byte)(res[48+96:]) = m.VotingPower.Bytes32() // big endian!
	return res
}

func DeserializeMember(data [48 + 96 + 32]byte) (Member, error) {
	var m Member
	var err error
	m.PublicKey, err = bls.DeserializePublicKey(*(*[48]byte)(data[0:]))
	if err != nil {
		return m, fmt.Errorf("invalid public key, %w", err)
	}
	m.ProofOfPossession, err = bls.DeserializeSignature(*(*[96]byte)(data[48:]))
	if err != nil {
		return m, fmt.Errorf("invalid proof of possession, %w", err)
	}
	m.VotingPower.SetBytes32(data[48+96:])
	return m, nil
}
