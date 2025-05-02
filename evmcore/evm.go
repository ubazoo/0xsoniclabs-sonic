// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package evmcore

//go:generate mockgen -source=evm.go -destination=evm_mock.go -package=evmcore

import (
	"math/big"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
)

// BlockHashProvider is an interface for retrieving block hashes.
type BlockHashProvider interface {
	// GetBlockHash returns the hash of the given block or an error if the block
	// does not exist. This is required in the EVM for the BLOCKHASH opcode.
	GetBlockHash(uint64) common.Hash
}

// NewEVMBlockContext creates a new context for use in the EVM.
func NewEVMBlockContext(header *EvmHeader, chain BlockHashProvider, author *common.Address) vm.BlockContext {
	var (
		beneficiary common.Address
		baseFee     *big.Int
		random      *common.Hash
	)
	// If we don't have an explicit author (i.e. not mining), extract from the header
	if author == nil {
		beneficiary = header.Coinbase
	} else {
		beneficiary = *author
	}
	if header.BaseFee != nil {
		baseFee = new(big.Int).Set(header.BaseFee)
	}

	// For legacy Opera network, the difficulty is always 1 and random nil
	difficulty := big.NewInt(1)
	if header.PrevRandao.Cmp(common.Hash{}) != 0 {
		// Difficulty must be set to 0 when PREVRANDAO is enabled.
		random = &header.PrevRandao
		difficulty.SetUint64(0)
	}
	return vm.BlockContext{
		CanTransfer: CanTransfer,
		Transfer:    Transfer,
		GetHash:     chain.GetBlockHash,
		Coinbase:    beneficiary,
		BlockNumber: new(big.Int).Set(header.Number),
		Time:        uint64(header.Time.Unix()),
		Difficulty:  difficulty,
		BaseFee:     baseFee,
		GasLimit:    header.GasLimit,
		Random:      random,
		BlobBaseFee: big.NewInt(1), // TODO issue #147
	}
}

// NewEVMTxContext creates a new transaction context for a single transaction.
func NewEVMTxContext(msg *core.Message) vm.TxContext {
	return vm.TxContext{
		Origin:     msg.From,
		GasPrice:   new(big.Int).Set(msg.GasPrice),
		BlobFeeCap: msg.BlobGasFeeCap,
	}
}

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db vm.StateDB, addr common.Address, amount *uint256.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db vm.StateDB, sender, recipient common.Address, amount *uint256.Int) {
	db.SubBalance(sender, amount, tracing.BalanceChangeTransfer)
	db.AddBalance(recipient, amount, tracing.BalanceChangeTransfer)
}
