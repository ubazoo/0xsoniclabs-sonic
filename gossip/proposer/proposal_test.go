package proposer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProposal_HashCoversContent(t *testing.T) {
	pb := &Proposal{
		Number:     1,
		ParentHash: [32]byte{1},
		Timestamp:  2,
		PrevRandao: [32]byte{3},
	}

	hash := pb.Hash()

	if hash != [32]byte{4} {
		t.Errorf("expected hash to be [32]byte{4}, got %v", hash)
	}
}

func TestProposal_CanBeSerializedAndRestored(t *testing.T) {
	require := require.New(t)
	original := &Proposal{
		Number:     1,
		ParentHash: [32]byte{2},
		Timestamp:  3,
		PrevRandao: [32]byte{4},
		// TODO: add transactions
	}

	data, err := original.Serialize()
	require.NoError(err)
	require.NotEmpty(data)
	restored := &Proposal{}
	err = restored.Deserialize(data)
	require.NoError(err)
	require.Equal(original, restored)
}
