package basiccheck

import (
	"errors"
	"math"

	"github.com/0xsoniclabs/sonic/eventcheck/basiccheck/legacy"
	"github.com/0xsoniclabs/sonic/evmcore"
	"github.com/0xsoniclabs/sonic/inter"

	base "github.com/Fantom-foundation/lachesis-base/eventcheck/basiccheck"
)

var (
	ErrWrongNetForkID = errors.New("wrong network fork ID")
	ErrZeroTime       = errors.New("event has zero timestamp")
)

type Checker struct {
	stateReader evmcore.StateReader

	baseChecker   *base.Checker
	legacyChecker *legacy.BasicCheck
}

func New(stateReader evmcore.StateReader) *Checker {
	return &Checker{
		stateReader:   stateReader,
		baseChecker:   base.New(),
		legacyChecker: legacy.NewChecker(),
	}
}

// Validate validates the event payload, according to the current chain rules.
func (c *Checker) Validate(e inter.EventPayloadI) error {

	// base checks
	if e.NetForkID() != 0 {
		return ErrWrongNetForkID
	}
	if err := c.baseChecker.Validate(e); err != nil {
		return err
	}
	if e.GasPowerUsed() >= math.MaxInt64-1 || e.GasPowerLeft().Max() >= math.MaxInt64-1 {
		return base.ErrHugeValue
	}
	if e.CreationTime() <= 0 || e.MedianTime() <= 0 {
		return ErrZeroTime
	}

	// payload checks
	config := c.stateReader.Config()
	currentBlock := c.stateReader.CurrentBlock()
	if config.IsPrague(currentBlock.Number, (uint64)(currentBlock.Time.Unix())) {
		return validateEventPayload(c.stateReader, e)
	} else {
		return c.legacyChecker.Validate(e)
	}
}

func validateEventPayload(state evmcore.StateReader, e inter.EventPayloadI) error {
	for _, tx := range e.Txs() {
		// TODO https://github.com/0xsoniclabs/sonic-admin/issues/141:
		// perform full validation of each transaction
		_ = tx
	}
	return nil
}
