package inter

import (
	"crypto/sha256"
	"encoding/binary"

	"github.com/0xsoniclabs/sonic/inter/pb"
	"github.com/Fantom-foundation/lachesis-base/hash"
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
	Number       uint64
	ParentHash   common.Hash
	Timestamp    Timestamp
	PrevRandao   common.Hash
	Transactions []*types.Transaction
	// TODO: consider adding fields needed for light client protocol
	// TODO: need a way to prevent a single validator to propose a huge block
}

// Hash computes a cryptographic hash of the proposal. The hash can be used to
// sign and verify the proposal.
func (p *Proposal) Hash() hash.Hash {
	size := 8 + 32 + 8 + 32 + 32*len(p.Transactions)
	data := make([]byte, 0, size)
	data = binary.BigEndian.AppendUint64(data, p.Number)
	data = append(data, p.ParentHash[:]...)
	data = binary.BigEndian.AppendUint64(data, uint64(p.Timestamp))
	data = append(data, p.PrevRandao[:]...)
	for _, tx := range p.Transactions {
		txHash := tx.Hash()
		data = append(data, txHash[:]...)
	}
	return sha256.Sum256(data)
}

func (p *Proposal) Serialize() ([]byte, error) {
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

	return proto.Marshal(&pb.Proposal{
		Number:       p.Number,
		ParentHash:   p.ParentHash[:],
		Timestamp:    uint64(p.Timestamp),
		PrevRandao:   p.PrevRandao[:],
		Transactions: transactions,
	})
}

func (p *Proposal) Deserialize(data []byte) error {
	var pb pb.Proposal
	if err := proto.Unmarshal(data, &pb); err != nil {
		return err
	}

	// Restore individual fields.
	p.Number = pb.Number
	copy(p.ParentHash[:], pb.ParentHash)
	p.Timestamp = Timestamp(pb.Timestamp)
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
