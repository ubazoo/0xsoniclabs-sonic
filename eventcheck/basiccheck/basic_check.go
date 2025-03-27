package basiccheck

import (
	"errors"

	"github.com/0xsoniclabs/sonic/inter"
)

var (
	ErrWrongNetForkID = errors.New("wrong network fork ID")
	ErrZeroTime       = errors.New("event has zero timestamp")
	ErrNegativeValue  = errors.New("negative value")
	ErrIntrinsicGas   = errors.New("intrinsic gas too low")
	// ErrTipAboveFeeCap is a sanity error to ensure no one is able to specify a
	// transaction with a tip higher than the total fee cap.
	ErrTipAboveFeeCap = errors.New("max priority fee per gas higher than max fee per gas")
)

type Checker struct {
	LegacyChecker *LegacyChecker
}

func New() *Checker {
	return &Checker{
		LegacyChecker: NewLegacyChecker(),
	}
}

// Validate event
func (v *Checker) Validate(e inter.EventPayloadI) error {

	// TODO: depending on the block height, use the validation needed
	validator := v.LegacyChecker

	return validator.Validate(e)
}
