package inter

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/0xsoniclabs/sonic/inter/pb"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/protobuf/proto"
)

type ProposalEnvelope struct {
	LastSeenProposalNumber  idx.Block
	LastSeenProposalAttempt uint32
	LastSeenProposalFrame   idx.Frame
	Proposal                *Proposal
}

func (e *ProposalEnvelope) Hash() hash.Hash {
	size := 8 + 4 + 4 + 32
	data := make([]byte, 0, size)
	data = binary.BigEndian.AppendUint64(data, uint64(e.LastSeenProposalNumber))
	data = binary.BigEndian.AppendUint32(data, e.LastSeenProposalAttempt)
	data = binary.BigEndian.AppendUint32(data, uint32(e.LastSeenProposalFrame))
	if e.Proposal != nil {
		hash := e.Proposal.Hash()
		data = append(data, hash[:]...)
	}
	return sha256.Sum256(data)
}

func (e *ProposalEnvelope) Serialize() ([]byte, error) {
	var proposal *pb.Proposal
	if e.Proposal != nil {
		p, err := e.Proposal.toProto()
		if err != nil {
			return nil, err
		}
		proposal = p
	}
	return proto.Marshal(&pb.ProposalEnvelope{
		LastSeenProposalNumber:  uint64(e.LastSeenProposalNumber),
		LastSeenProposalAttempt: e.LastSeenProposalAttempt,
		LastSeenProposalFrame:   uint32(e.LastSeenProposalFrame),
		Proposal:                proposal,
	})
}

func (e *ProposalEnvelope) Deserialize(data []byte) error {
	var pb pb.ProposalEnvelope
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}
	e.LastSeenProposalNumber = idx.Block(pb.LastSeenProposalNumber)
	e.LastSeenProposalAttempt = pb.LastSeenProposalAttempt
	e.LastSeenProposalFrame = idx.Frame(pb.LastSeenProposalFrame)
	if pb.Proposal != nil {
		p := &Proposal{}
		if err := p.fromProto(pb.Proposal); err != nil {
			return err
		}
		e.Proposal = p
	} else {
		e.Proposal = nil
	}
	return nil
}

// Proposal represents a block proposal in the consensus protocol.
// It contains the block number, parent hash, timestamp, previous randao,
// and a list of transactions.
//
// A proposal is a candidate for inclusion in the blockchain and is
// created by a proposer. It is signed by the proposer and sent to
// validators for validation and inclusion in the blockchain.
type Proposal struct {
	Number       idx.Block
	Attempt      uint32
	ParentHash   common.Hash
	Time         Timestamp
	PrevRandao   common.Hash
	Transactions []*types.Transaction
	// TODO: consider adding fields needed for light client protocol
	// TODO: need a way to prevent a single validator to propose a huge block
}

// Hash computes a cryptographic hash of the proposal. The hash can be used to
// sign and verify the proposal.
func (p *Proposal) Hash() hash.Hash {
	size := 8 + 4 + 32 + 8 + 32 + 32*len(p.Transactions)
	data := make([]byte, 0, size)
	data = binary.BigEndian.AppendUint64(data, uint64(p.Number))
	data = binary.BigEndian.AppendUint32(data, uint32(p.Attempt))
	data = append(data, p.ParentHash[:]...)
	data = binary.BigEndian.AppendUint64(data, uint64(p.Time))
	data = append(data, p.PrevRandao[:]...)
	for _, tx := range p.Transactions {
		txHash := tx.Hash()
		data = append(data, txHash[:]...)
	}
	return sha256.Sum256(data)
}

func (p *Proposal) Serialize() ([]byte, error) {
	pb, err := p.toProto()
	if err != nil {
		return nil, err
	}
	return proto.Marshal(pb)
}

func (p *Proposal) Deserialize(data []byte) error {
	var pb pb.Proposal
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}
	return p.fromProto(&pb)
}

func (p *Proposal) toProto() (*pb.Proposal, error) {
	transactions := make([]*pb.Transaction, 0, len(p.Transactions))
	for _, tx := range p.Transactions {
		data, err := tx.MarshalBinary()
		if err != nil {
			return nil, err
		}
		transactions = append(transactions, &pb.Transaction{
			Encoded: data,
		})
	}

	return &pb.Proposal{
		Number:       uint64(p.Number),
		Attempt:      p.Attempt,
		ParentHash:   p.ParentHash[:],
		Timestamp:    uint64(p.Time),
		PrevRandao:   p.PrevRandao[:],
		Transactions: transactions,
	}, nil
}

func (p *Proposal) fromProto(pb *pb.Proposal) error {
	// Restore individual fields.
	p.Number = idx.Block(pb.Number)
	p.Attempt = pb.Attempt
	copy(p.ParentHash[:], pb.ParentHash)
	p.Time = Timestamp(pb.Timestamp)
	copy(p.PrevRandao[:], pb.PrevRandao)
	for _, tx := range pb.Transactions {
		var transaction types.Transaction
		if err := transaction.UnmarshalBinary(tx.Encoded); err != nil {
			return err
		}
		p.Transactions = append(p.Transactions, &transaction)
	}

	return nil
}
