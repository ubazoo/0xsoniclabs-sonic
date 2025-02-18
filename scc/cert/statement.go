package cert

import (
	"github.com/0xsoniclabs/sonic/scc"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
)

// Statement is a claim that can be signed, attested, and/or certified.
type Statement interface {
	GetStatementHash() common.Hash
}

type statement struct {
	ChainId uint256.Int
}

// BlockStatement is a statement that a block on a given chain with a certain
// number has a certain hash and state root.
type BlockStatement struct {
	statement
	Number    uint64
	Hash      common.Hash
	StateRoot common.Hash
}

type CommitteeStatement struct {
	statement
	Period    uint64
	Committee scc.Committee
}
