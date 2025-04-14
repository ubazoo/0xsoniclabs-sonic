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
	Randao       common.Hash
	Transactions []*types.Transaction
}

// Hash computes a cryptographic hash of the proposal. The hash can be used to
// sign and verify the proposal.
func (p *Proposal) Hash() hash.Hash {
	data := []byte{}
	data = binary.BigEndian.AppendUint64(data, uint64(p.Number))
	data = binary.BigEndian.AppendUint32(data, uint32(p.Attempt))
	data = append(data, p.ParentHash[:]...)
	data = binary.BigEndian.AppendUint64(data, uint64(p.Time))
	data = append(data, p.Randao[:]...)
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
		Randao:       p.Randao[:],
		Transactions: transactions,
	}, nil
}

func (p *Proposal) fromProto(pb *pb.Proposal) error {
	// Restore individual fields.
	p.Number = idx.Block(pb.Number)
	p.Attempt = pb.Attempt
	copy(p.ParentHash[:], pb.ParentHash)
	p.Time = Timestamp(pb.Timestamp)
	copy(p.Randao[:], pb.Randao)
	for _, tx := range pb.Transactions {
		var transaction types.Transaction
		if err := transaction.UnmarshalBinary(tx.Encoded); err != nil {
			return err
		}
		p.Transactions = append(p.Transactions, &transaction)
	}

	return nil
}
