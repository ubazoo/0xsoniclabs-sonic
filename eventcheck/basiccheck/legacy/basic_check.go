package legacy

import (
	"errors"

	"github.com/ethereum/go-ethereum/core/types"

	"github.com/0xsoniclabs/sonic/inter"
)

var (
	ErrNegativeValue = errors.New("negative value")
	ErrIntrinsicGas  = errors.New("intrinsic gas too low")
	// ErrTipAboveFeeCap is a sanity error to ensure no one is able to specify a
	// transaction with a tip higher than the total fee cap.
	ErrTipAboveFeeCap = errors.New("max priority fee per gas higher than max fee per gas")
)

type BasicCheck struct {
}

// New validator which performs checks which don't require anything except event
func NewChecker() *BasicCheck {
	return &BasicCheck{}
}

// validateTx checks whether a transaction is valid according to the consensus
// rules
func validateTx(tx *types.Transaction) error {
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	if tx.Value().Sign() < 0 || tx.GasPrice().Sign() < 0 {
		return ErrNegativeValue
	}
	// Ensure the transaction has more gas than the basic tx fee.
	intrGas, err := IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}

	if tx.GasFeeCapIntCmp(tx.GasTipCap()) < 0 {
		return ErrTipAboveFeeCap
	}
	return nil
}

func (v *BasicCheck) checkTxs(e inter.EventPayloadI) error {
	for _, tx := range e.Txs() {
		if err := validateTx(tx); err != nil {
			return err
		}
	}
	return nil
}

// Validate event
func (v *BasicCheck) Validate(e inter.EventPayloadI) error {
	if err := v.checkTxs(e); err != nil {
		return err
	}

	return nil
}
