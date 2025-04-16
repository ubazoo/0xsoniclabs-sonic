package inter

import (
	"crypto/sha256"
	"encoding/binary"
	"testing"

	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestProposalEnvelope_Hash_IsShaOfFieldConcatenation(t *testing.T) {
	envelope := &ProposalEnvelope{
		LastSeenProposalTurn:  1,
		LastSeenProposedBlock: 2,
		LastSeenProposalFrame: 3,
	}

	data := []byte{}
	data = binary.BigEndian.AppendUint32(data, uint32(envelope.LastSeenProposalTurn))
	data = binary.BigEndian.AppendUint64(data, uint64(envelope.LastSeenProposedBlock))
	data = binary.BigEndian.AppendUint32(data, uint32(envelope.LastSeenProposalFrame))
	require.Equal(t, hash.Hash(sha256.Sum256(data)), envelope.Hash())

	envelope.Proposal = &Proposal{
		Number: 4,
	}
	proposalHash := envelope.Proposal.Hash()
	data = append(data, proposalHash[:]...)
	require.Equal(t, hash.Hash(sha256.Sum256(data)), envelope.Hash())
}

func TestProposalEnvelope_CanBeSerializedAndRestored(t *testing.T) {
	for _, proposal := range []*Proposal{nil, {}} {
		require := require.New(t)
		original := &ProposalEnvelope{
			LastSeenProposalTurn:  1,
			LastSeenProposedBlock: 2,
			LastSeenProposalFrame: 3,
			Proposal:              proposal,
		}

		data, err := original.Serialize()
		require.NoError(err)
		require.NotEmpty(data)
		restored := &ProposalEnvelope{}
		err = restored.Deserialize(data)
		require.NoError(err)

		// Check individual fields. Note: a full Deep-Equal comparison is not
		// possible because transactions have insignificant meta-information that
		// is not serialized and restored.
		require.Equal(original.LastSeenProposalTurn, restored.LastSeenProposalTurn)
		require.Equal(original.LastSeenProposedBlock, restored.LastSeenProposedBlock)
		require.Equal(original.LastSeenProposalFrame, restored.LastSeenProposalFrame)

		if original.Proposal == nil {
			require.Nil(restored.Proposal)
		} else {
			require.NotNil(restored.Proposal)
			require.Equal(original.Proposal.Number, restored.Proposal.Number)
		}

		require.Equal(original.Hash(), restored.Hash())
	}
}

func TestProposal_Hash_IsShaOfFieldConcatenation(t *testing.T) {

	// The procedure of computing the hash of a proposal is critical for
	// the consensus protocol. It is important to ensure that the hash is
	// computed correctly and consistently. Thus, this test provides an
	// second implementation of the hash function. If you have to change this
	// test, make sure you understand the implications on consensus.

	// TODO: add a fuzzer test for this
	proposal := &Proposal{
		Number:     2,
		ParentHash: [32]byte{0: 1, 31: 2},
		Time:       3,
		PrevRandao: [32]byte{0: 3, 21: 4},
		Transactions: []*types.Transaction{
			types.NewTx(&types.LegacyTx{Nonce: 1}),
			types.NewTx(&types.LegacyTx{Nonce: 2}),
		},
	}

	hash := func(proposal *Proposal) hash.Hash {
		data := []byte{}
		data = binary.BigEndian.AppendUint64(data, uint64(proposal.Number))
		data = append(data, proposal.ParentHash[:]...)
		data = binary.BigEndian.AppendUint64(data, uint64(proposal.Time))
		data = append(data, proposal.PrevRandao[:]...)
		for _, tx := range proposal.Transactions {
			txHash := tx.Hash()
			data = append(data, txHash[:]...)
		}
		return sha256.Sum256(data)
	}

	require.Equal(t, proposal.Hash(), hash(proposal))
}

func TestProposal_Hash_ModifyingContent_ChangesHash(t *testing.T) {
	tests := map[string]func(*Proposal){
		"change number": func(p *Proposal) {
			p.Number = p.Number + 1
		},
		"change parent hash": func(p *Proposal) {
			p.ParentHash[0] = p.ParentHash[0] + 1
		},
		"change timestamp": func(p *Proposal) {
			p.Time = p.Time + 1
		},
		"change prev randao": func(p *Proposal) {
			p.PrevRandao[0] = p.PrevRandao[0] + 1
		},
		"change transaction": func(p *Proposal) {
			p.Transactions[0] = types.NewTx(&types.LegacyTx{Nonce: 3})
		},
		"add transaction": func(p *Proposal) {
			p.Transactions = append(p.Transactions, types.NewTx(
				&types.LegacyTx{Nonce: 4},
			))
		},
		"remove transaction": func(p *Proposal) {
			p.Transactions = p.Transactions[:len(p.Transactions)-1]
		},
	}

	for name, modify := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)
			proposal := &Proposal{
				Number:     1,
				ParentHash: [32]byte{1},
				Time:       2,
				PrevRandao: [32]byte{3},
				Transactions: []*types.Transaction{
					types.NewTx(&types.LegacyTx{Nonce: 1}),
					types.NewTx(&types.LegacyTx{Nonce: 2}),
				},
			}

			hashBefore := proposal.Hash()
			modify(proposal)
			hashAfter := proposal.Hash()

			require.NotEqual(hashBefore, hashAfter)
		})
	}
}

func TestProposal_CanBeSerializedAndRestored(t *testing.T) {
	require := require.New(t)
	original := &Proposal{
		Number:     1,
		ParentHash: [32]byte{2},
		Time:       3,
		PrevRandao: [32]byte{4},
		Transactions: []*types.Transaction{
			types.NewTx(&types.LegacyTx{Nonce: 1}),
			types.NewTx(&types.LegacyTx{Nonce: 2}),
		},
	}

	data, err := original.Serialize()
	require.NoError(err)
	require.NotEmpty(data)
	restored := &Proposal{}
	err = restored.Deserialize(data)
	require.NoError(err)

	// Check individual fields. Note: a full Deep-Equal comparison is not
	// possible because transactions have insignificant meta-information that
	// is not serialized and restored.
	require.Equal(original.Number, restored.Number)
	require.Equal(original.ParentHash, restored.ParentHash)
	require.Equal(original.Time, restored.Time)
	require.Equal(original.PrevRandao, restored.PrevRandao)

	require.Equal(len(original.Transactions), len(restored.Transactions))
	for i := range original.Transactions {
		require.Equal(original.Transactions[i].Hash(), restored.Transactions[i].Hash())
	}

	require.Equal(original.Hash(), restored.Hash())
}
