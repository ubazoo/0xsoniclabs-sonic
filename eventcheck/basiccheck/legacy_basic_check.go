package basiccheck

import (
	"math"

	base "github.com/Fantom-foundation/lachesis-base/eventcheck/basiccheck"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/0xsoniclabs/sonic/eventcheck/basiccheck/legacy"
	"github.com/0xsoniclabs/sonic/inter"
)

type LegacyChecker struct {
	base base.Checker
}

// New validator which performs checks which don't require anything except event
func NewLegacyChecker() *LegacyChecker {
	return &LegacyChecker{
		base: base.Checker{},
	}
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
	intrGas, err := legacy.IntrinsicGas(tx.Data(), tx.AccessList(), tx.To() == nil)
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

func (v *LegacyChecker) checkTxs(e inter.EventPayloadI) error {
	for _, tx := range e.Txs() {
		if err := validateTx(tx); err != nil {
			return err
		}
	}
	return nil
}

// Validate event
func (v *LegacyChecker) Validate(e inter.EventPayloadI) error {
	if e.NetForkID() != 0 {
		return ErrWrongNetForkID
	}
	if err := v.base.Validate(e); err != nil {
		return err
	}
	if e.GasPowerUsed() >= math.MaxInt64-1 || e.GasPowerLeft().Max() >= math.MaxInt64-1 {
		return base.ErrHugeValue
	}
	if e.CreationTime() <= 0 || e.MedianTime() <= 0 {
		return ErrZeroTime
	}
	if err := v.checkTxs(e); err != nil {
		return err
	}

	return nil
}
