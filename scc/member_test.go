package scc

import (
	"strings"
	"testing"

	"github.com/0xsoniclabs/sonic/scc/bls"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestMember_Default_IsInvalid(t *testing.T) {
	require.Error(t, Member{}.Validate())
}

func TestMember_Validate_AcceptsValidMembers(t *testing.T) {
	key := bls.NewPrivateKey()
	pub := key.PublicKey()
	proof := key.GetProofOfPossession()

	tests := map[string]Member{
		"regular": {
			PublicKey:         pub,
			ProofOfPossession: proof,
			VotingPower:       *uint256.NewInt(12),
		},
		"huge voting power": {
			PublicKey:         pub,
			ProofOfPossession: proof,
			VotingPower:       *new(uint256.Int).Lsh(uint256.NewInt(1), 255),
		},
	}

	for name, m := range tests {
		t.Run(name, func(t *testing.T) {
			if err := m.Validate(); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestMember_Validate_DetectsInvalidMembers(t *testing.T) {
	key := bls.NewPrivateKey()
	pub := key.PublicKey()
	proof := key.GetProofOfPossession()

	tests := map[string]Member{
		"invalid public key": {
			PublicKey:         bls.PublicKey{},
			ProofOfPossession: proof,
			VotingPower:       *uint256.NewInt(12),
		},
		"invalid proof of possession": {
			PublicKey:         pub,
			ProofOfPossession: bls.Signature{},
			VotingPower:       *uint256.NewInt(12),
		},
		"zero voting power": {
			PublicKey:         pub,
			ProofOfPossession: proof,
			VotingPower:       *uint256.NewInt(0),
		},
	}

	for name, m := range tests {
		t.Run(name, func(t *testing.T) {
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), name) {
				t.Errorf("expected error, got %v", err)
			}
		})
	}
}

func TestMember_Serialization_CanEncodeAndDecodeMember(t *testing.T) {
	key := bls.NewPrivateKey()
	original := Member{
		PublicKey:         key.PublicKey(),
		ProofOfPossession: key.GetProofOfPossession(),
		VotingPower:       *uint256.NewInt(12),
	}
	recovered, err := DeserializeMember(original.Serialize())
	require.NoError(t, err)
	require.Equal(t, original, recovered)
}

func TestMember_Deserialize_DetectsEncodingErrors(t *testing.T) {
	encoded := [176]byte{}
	_, err := DeserializeMember(encoded)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid public key")

	key := bls.NewPrivateKey()
	*(*[48]byte)(encoded[:]) = key.PublicKey().Serialize()

	_, err = DeserializeMember(encoded)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid proof of possession")
}
