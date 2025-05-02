package emitter

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/0xsoniclabs/sonic/gossip/emitter/scheduler"
	"github.com/0xsoniclabs/sonic/inter"
	"github.com/0xsoniclabs/sonic/inter/state"
	"github.com/0xsoniclabs/sonic/opera"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/dag"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestEmitter_addProposal_ProducesValidProposal(t *testing.T) {
	// This is an integration test performing a smoke-test for the combination
	// of the various steps required to build a block proposal. Details are
	// covered by the tests below.
	ctrl := gomock.NewController(t)
	world := NewMockExternal(ctrl)

	thisValidator := idx.ValidatorID(1)
	builder := pos.ValidatorsBuilder{}
	builder.Set(thisValidator, 10)
	validators := builder.Build()

	emitter := &Emitter{
		world:      World{External: world},
		validators: validators,
		config: Config{
			Validator: ValidatorConfig{
				ID: thisValidator,
			},
		},
	}

	latestBlock := inter.NewBlockBuilder().WithNumber(12).Build()

	state := state.NewMockStateDB(ctrl)

	rules := opera.Rules{
		Economy: opera.EconomyRules{
			ShortGasPower: opera.GasPowerRules{
				AllocPerSec: 100_000_000,
			},
		},
		Blocks: opera.BlocksRules{
			MaxBlockGas: 100_000_000,
		},
	}

	any := gomock.Any()
	world.EXPECT().GetEpochStartBlock(any).Return(idx.Block(12))
	world.EXPECT().GetLatestBlock().Return(latestBlock)
	world.EXPECT().GetRules().Return(rules).AnyTimes()
	world.EXPECT().GetUpgradeHeights().Return([]opera.UpgradeHeight{}).AnyTimes()
	world.EXPECT().StateDB().Return(state).AnyTimes()

	// The candidate transaction is executed during scheduling.
	state.EXPECT().SetTxContext(any, any).AnyTimes()
	state.EXPECT().GetBalance(any).Return(uint256.NewInt(100)).AnyTimes()
	state.EXPECT().SubBalance(any, any, any).AnyTimes()
	state.EXPECT().AddBalance(any, any, any).AnyTimes()
	state.EXPECT().GetNonce(any).Return(uint64(1)).AnyTimes()
	state.EXPECT().SetNonce(any, any, any).AnyTimes()
	state.EXPECT().GetCode(any).Return(nil).AnyTimes()
	state.EXPECT().Snapshot().AnyTimes()
	state.EXPECT().Exist(any).Return(true).AnyTimes()
	state.EXPECT().GetRefund().AnyTimes()
	state.EXPECT().GetLogs(any, any).AnyTimes()
	state.EXPECT().Prepare(any, any, any, any, any, any).AnyTimes()
	state.EXPECT().EndTransaction().AnyTimes()
	state.EXPECT().TxIndex().AnyTimes()
	state.EXPECT().Release().AnyTimes()

	tx := types.NewTx(&types.LegacyTx{
		To:    &common.Address{},
		Nonce: 1,
		Gas:   21_000,
	})

	sorted := newTransactionsByPriceAndNonce(
		types.NewLondonSigner(big.NewInt(12)),
		map[common.Address][]*txpool.LazyTransaction{
			common.Address{1}: {
				&txpool.LazyTransaction{
					Tx:        tx,
					GasFeeCap: new(uint256.Int).SetUint64(100),
					GasTipCap: new(uint256.Int).SetUint64(10),
				},
			},
		},
		big.NewInt(0), // < base fee
	)

	event := inter.NewMockEventI(ctrl)
	event.EXPECT().Parents().Return(hash.Events{})
	event.EXPECT().Epoch().Return(idx.Epoch(3)).AnyTimes()
	event.EXPECT().Frame().Return(idx.Frame(2))
	event.EXPECT().MedianTime().Return(inter.Timestamp(1 * time.Second))

	payload, err := emitter.createPayload(event, sorted)
	require.NoError(t, err)
	require.Equal(t, inter.Turn(1), payload.LastSeenProposalTurn)
	require.Equal(t, idx.Frame(2), payload.LastSeenProposalFrame)
	require.Equal(t, idx.Block(13), payload.LastSeenProposedBlock)
	require.Equal(t, idx.Block(13), payload.Proposal.Number)
	require.Equal(t, latestBlock.Hash(), payload.Proposal.ParentHash)
	require.Equal(t, []*types.Transaction{tx}, payload.Proposal.Transactions)
}

func TestEmitter_addProposal_FailsIfThereAreNoValidators(t *testing.T) {
	ctrl := gomock.NewController(t)
	world := NewMockExternal(ctrl)

	emitter := &Emitter{
		world:      World{External: world},
		validators: pos.ValidatorsBuilder{}.Build(), // < this makes it fail
	}

	latestBlock := inter.NewBlockBuilder().WithNumber(12).Build()

	any := gomock.Any()
	world.EXPECT().GetEpochStartBlock(any).Return(idx.Block(12))
	world.EXPECT().GetLatestBlock().Return(latestBlock)

	event := &inter.MutableEventPayload{}
	_, err := emitter.createPayload(event, nil)
	require.ErrorContains(t, err, "no validators")
}

func TestCreatePayload_InvalidTurn_CreatesEmptyPayload(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockworldReader(ctrl)
	event := inter.NewMockEventI(ctrl)

	p1 := hash.Event{1}
	p2 := hash.Event{2}
	payloads := map[hash.Event]inter.Payload{
		p1: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(0x01),
			LastSeenProposalFrame: idx.Frame(0x12),
			LastSeenProposedBlock: idx.Block(0x23),
		},
		p2: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(0x03),
			LastSeenProposalFrame: idx.Frame(0x11),
			LastSeenProposedBlock: idx.Block(0x22),
		},
	}

	world.EXPECT().GetEventPayload(p1).Return(payloads[p1])
	world.EXPECT().GetEventPayload(p2).Return(payloads[p2])

	world.EXPECT().GetLatestBlock().Return(
		inter.NewBlockBuilder().
			WithNumber(5).Build(),
	)

	event.EXPECT().Parents().Return(hash.Events{p1, p2})
	event.EXPECT().Frame().Return(idx.Frame(0x13))

	// This call fails since it tries to propose block 6 while according to the
	// parent events, proposals for 0x23 have already been made.
	payload, err := createPayload(
		world, nil, 0, event, nil, nil,
	)

	var a, b proposalState
	a.fromPayload(payloads[p1])
	b.fromPayload(payloads[p2])
	want := joinProposalState(a, b).toPayload()

	require.NoError(err)
	require.Equal(want, payload)
}

func TestCreatePayload_InvalidValidators_ForwardsError(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)

	event := inter.NewMockEventI(ctrl)
	event.EXPECT().Parents().Return(hash.Events{})
	event.EXPECT().Epoch().Return(idx.Epoch(12)).AnyTimes()
	event.EXPECT().Frame().Return(idx.Frame(0))

	world := NewMockworldReader(ctrl)
	world.EXPECT().GetEpochStartBlock(event.Epoch()).Return(idx.Block(62))
	world.EXPECT().GetLatestBlock().Return(
		inter.NewBlockBuilder().WithNumber(62).Build(),
	)

	validators := pos.ValidatorsBuilder{}.Build()

	_, err := createPayload(
		world, validators, 0, event, nil, nil,
	)
	require.ErrorContains(err, "no validators")
}

func TestCreatePayload_ValidTurn_ProducesExpectedPayload(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockworldReader(ctrl)
	event := inter.NewMockEventI(ctrl)

	p1 := hash.Event{1}
	p2 := hash.Event{2}
	payloads := map[hash.Event]inter.Payload{
		p1: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(1),
			LastSeenProposalFrame: idx.Frame(2),
			LastSeenProposedBlock: idx.Block(5),
		},
		p2: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(1),
			LastSeenProposalFrame: idx.Frame(2),
			LastSeenProposedBlock: idx.Block(5),
		},
	}

	world.EXPECT().GetEventPayload(p1).Return(payloads[p1])
	world.EXPECT().GetEventPayload(p2).Return(payloads[p2])

	world.EXPECT().GetLatestBlock().Return(
		inter.NewBlockBuilder().
			WithNumber(5).Build(),
	)

	world.EXPECT().GetRules().Return(opera.Rules{})

	event.EXPECT().Parents().Return(hash.Events{p1, p2})
	event.EXPECT().Frame().Return(idx.Frame(4)).AnyTimes()
	event.EXPECT().MedianTime().Return(inter.Timestamp(1234))

	thisValidator := idx.ValidatorID(1)
	builder := pos.ValidatorsBuilder{}
	builder.Set(thisValidator, 10)
	validators := builder.Build()

	txs := []*types.Transaction{
		types.NewTx(&types.LegacyTx{Nonce: 1}),
		types.NewTx(&types.LegacyTx{Nonce: 2}),
	}

	any := gomock.Any()
	scheduler := NewMocktxScheduler(ctrl)
	scheduler.EXPECT().Schedule(any, any, any, any).Return(txs)

	payload, err := createPayload(
		world, validators, thisValidator, event, nil, scheduler,
	)
	require.NoError(err)

	require.Equal(inter.Turn(2), payload.LastSeenProposalTurn)
	require.Equal(idx.Frame(4), payload.LastSeenProposalFrame)
	require.Equal(idx.Block(6), payload.LastSeenProposedBlock)
	require.Equal(idx.Block(6), payload.Proposal.Number)
	require.Equal(inter.Timestamp(1234), payload.Proposal.Time)
	require.Equal(txs, payload.Proposal.Transactions)
}

func TestWorldReaderAdapter_ForwardsCallsToWrappedWorld(t *testing.T) {
	t.Run("GetLatestBlock", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		world := NewMockExternal(ctrl)
		adapter := worldReaderAdapter{world}

		block := inter.NewBlockBuilder().WithNumber(5).Build()
		world.EXPECT().GetLatestBlock().Return(block)

		require.Equal(t, block, adapter.GetLatestBlock())
	})

	t.Run("GetEpochStartBlock", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		world := NewMockExternal(ctrl)
		adapter := worldReaderAdapter{world}

		block := idx.Block(5)
		world.EXPECT().GetEpochStartBlock(idx.Epoch(12)).Return(block)

		require.Equal(t, block, adapter.GetEpochStartBlock(idx.Epoch(12)))
	})

	t.Run("GetEventPayload", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		world := NewMockExternal(ctrl)
		adapter := worldReaderAdapter{world}

		payload := inter.Payload{
			LastSeenProposalTurn:  inter.Turn(1),
			LastSeenProposalFrame: idx.Frame(2),
			LastSeenProposedBlock: idx.Block(5),
		}

		event := hash.Event{1}
		builder := inter.MutableEventPayload{}
		builder.SetPayload(payload)
		eventPayload := builder.Build()
		world.EXPECT().GetEventPayload(event).Return(eventPayload)

		require.Equal(t, payload, adapter.GetEventPayload(event))
	})

	t.Run("GetRules", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		world := NewMockExternal(ctrl)
		adapter := worldReaderAdapter{world}
		rules := opera.Rules{}
		world.EXPECT().GetRules().Return(rules)
		require.Equal(t, rules, adapter.GetRules())
	})
}

func TestProposalState_FromPayload(t *testing.T) {
	for turn := range inter.Turn(10) {
		for frame := range idx.Frame(10) {
			for block := range idx.Block(10) {
				state := proposalState{}
				state.fromPayload(inter.Payload{
					LastSeenProposalTurn:  turn,
					LastSeenProposalFrame: frame,
					LastSeenProposedBlock: block,
				})
				require.Equal(t, turn, state.lastSeenProposalTurn)
				require.Equal(t, frame, state.lastSeenProposalFrame)
				require.Equal(t, block, state.lastSeenProposedBlock)
			}
		}
	}
}

func TestProposalState_ToPayload(t *testing.T) {
	for turn := range inter.Turn(10) {
		for frame := range idx.Frame(10) {
			for block := range idx.Block(10) {
				payload := proposalState{
					lastSeenProposalTurn:  turn,
					lastSeenProposalFrame: frame,
					lastSeenProposedBlock: block,
				}.toPayload()
				require.Equal(t, turn, payload.LastSeenProposalTurn)
				require.Equal(t, frame, payload.LastSeenProposalFrame)
				require.Equal(t, block, payload.LastSeenProposedBlock)
			}
		}
	}
}

func TestProposalState_Join(t *testing.T) {
	for turnA := range inter.Turn(5) {
		for turnB := range inter.Turn(5) {
			for frameA := range idx.Frame(5) {
				for frameB := range idx.Frame(5) {
					for blockA := range idx.Block(5) {
						for blockB := range idx.Block(5) {
							a := proposalState{
								lastSeenProposalTurn:  turnA,
								lastSeenProposalFrame: frameA,
								lastSeenProposedBlock: blockA,
							}
							b := proposalState{
								lastSeenProposalTurn:  turnB,
								lastSeenProposalFrame: frameB,
								lastSeenProposedBlock: blockB,
							}
							joined := joinProposalState(a, b)
							require.Equal(t, max(turnA, turnB), joined.lastSeenProposalTurn)
							require.Equal(t, max(frameA, frameB), joined.lastSeenProposalFrame)
							require.Equal(t, max(blockA, blockB), joined.lastSeenProposedBlock)
						}
					}
				}
			}
		}
	}
}

func TestGetIncomingProposalState_ProducesEpochStartStateForGenesisEvent(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockeventReader(ctrl)

	event := &inter.MutableEventPayload{}
	event.SetEpoch(42)
	require.Empty(event.Parents())

	epochStartBlock := idx.Block(123)
	world.EXPECT().GetEpochStartBlock(event.Epoch()).Return(epochStartBlock)

	state := getIncomingProposalState(world, event)
	require.Equal(inter.Turn(0), state.lastSeenProposalTurn)
	require.Equal(idx.Frame(0), state.lastSeenProposalFrame)
	require.Equal(epochStartBlock, state.lastSeenProposedBlock)
}

func TestGetIncomingProposalState_AggregatesParentStates(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	world := NewMockeventReader(ctrl)

	p1 := hash.Event{1}
	p2 := hash.Event{2}
	p3 := hash.Event{3}
	parents := map[hash.Event]inter.Payload{
		p1: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(0x01),
			LastSeenProposalFrame: idx.Frame(0x12),
			LastSeenProposedBlock: idx.Block(0x23),
		},
		p2: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(0x03),
			LastSeenProposalFrame: idx.Frame(0x11),
			LastSeenProposedBlock: idx.Block(0x22),
		},
		p3: inter.Payload{
			LastSeenProposalTurn:  inter.Turn(0x02),
			LastSeenProposalFrame: idx.Frame(0x13),
			LastSeenProposedBlock: idx.Block(0x21),
		},
	}

	world.EXPECT().GetEventPayload(p1).Return(parents[p1])
	world.EXPECT().GetEventPayload(p2).Return(parents[p2])
	world.EXPECT().GetEventPayload(p3).Return(parents[p3])

	event := &dag.MutableBaseEvent{}
	event.SetParents(hash.Events{p1, p2, p3})
	state := getIncomingProposalState(world, event)

	require.Equal(inter.Turn(0x03), state.lastSeenProposalTurn)
	require.Equal(idx.Frame(0x13), state.lastSeenProposalFrame)
	require.Equal(idx.Block(0x23), state.lastSeenProposedBlock)
}

func TestIsAllowedToPropose_AcceptsValidProposerTurn(t *testing.T) {
	require := require.New(t)

	thisValidator := idx.ValidatorID(1)
	builder := pos.ValidatorsBuilder{}
	builder.Set(thisValidator, 10)
	validators := builder.Build()

	last := inter.ProposalSummary{
		Turn:  inter.Turn(5),
		Frame: idx.Frame(12),
	}
	next := inter.ProposalSummary{
		Turn:  inter.Turn(6),
		Frame: idx.Frame(17),
	}
	require.True(inter.IsValidTurnProgression(last, next))

	ok, err := isAllowedToPropose(
		proposalState{
			lastSeenProposalTurn:  last.Turn,
			lastSeenProposalFrame: last.Frame,
			lastSeenProposedBlock: idx.Block(4),
		},
		next.Frame,
		validators,
		thisValidator,
		5, // block to be proposed
	)
	require.NoError(err)
	require.True(ok)
}

func TestIsAllowedToPropose_RejectsInvalidProposerTurn(t *testing.T) {
	validatorA := idx.ValidatorID(1)
	validatorB := idx.ValidatorID(2)
	builder := pos.ValidatorsBuilder{}
	builder.Set(validatorA, 10)
	builder.Set(validatorB, 20)
	validators := builder.Build()

	validTurn := inter.Turn(5)
	validProposer, err := inter.GetProposer(validators, validTurn)
	require.NoError(t, err)
	invalidProposer := validatorA
	if invalidProposer == validProposer {
		invalidProposer = validatorB
	}

	type input struct {
		thisValidator     idx.ValidatorID
		blockToBeProposed idx.Block
		currentFrame      idx.Frame
	}

	tests := map[string]func(*input){
		"wrong proposer": func(input *input) {
			input.thisValidator = invalidProposer
		},
		"proposed block is too old": func(input *input) {
			input.blockToBeProposed = input.blockToBeProposed - 1
		},
		"proposed block is too new": func(input *input) {
			input.blockToBeProposed = input.blockToBeProposed + 1
		},
		"invalid turn progression": func(input *input) {
			// a proposal made too late needs to be rejected
			input.currentFrame = input.currentFrame * 10
		},
	}

	for name, corrupt := range tests {
		t.Run(name, func(t *testing.T) {
			require := require.New(t)

			proposalState := proposalState{
				lastSeenProposalTurn:  12,
				lastSeenProposalFrame: 62,
				lastSeenProposedBlock: 5,
			}

			input := input{
				currentFrame:      67,
				thisValidator:     validProposer,
				blockToBeProposed: 6,
			}

			ok, err := isAllowedToPropose(
				proposalState,
				input.currentFrame,
				validators,
				input.thisValidator,
				input.blockToBeProposed,
			)
			require.NoError(err)
			require.True(ok)

			corrupt(&input)
			ok, err = isAllowedToPropose(
				proposalState,
				input.currentFrame,
				validators,
				input.thisValidator,
				input.blockToBeProposed,
			)
			require.NoError(err)
			require.False(ok)
		})
	}
}

func TestIsAllowedToPropose_ForwardsTurnSelectionError(t *testing.T) {
	validators := pos.ValidatorsBuilder{}.Build()

	_, want := inter.GetProposer(validators, inter.Turn(0))
	require.Error(t, want)

	_, got := isAllowedToPropose(
		proposalState{},
		idx.Frame(0),
		validators,
		idx.ValidatorID(0),
		idx.Block(1),
	)
	require.Error(t, got)
	require.Equal(t, got, want)
}

func TestCreateProposal_ValidArguments_CreatesValidProposal(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	mockScheduler := NewMocktxScheduler(ctrl)
	durationMetric := NewMocktimerMetric(ctrl)
	timeoutMetric := NewMockcounterMetric(ctrl)

	rules := opera.Rules{}
	state := proposalState{
		lastSeenProposalTurn:  inter.Turn(5),
		lastSeenProposalFrame: idx.Frame(12),
		lastSeenProposedBlock: idx.Block(4),
	}
	latestBlock := inter.NewBlockBuilder().
		WithNumber(5).
		WithTime(1234).
		Build()

	delta := 20 * time.Millisecond
	newBlockTime := inter.Timestamp(1234) + inter.Timestamp(delta)
	currentFrame := idx.Frame(17)

	transactions := []*types.Transaction{
		types.NewTx(&types.LegacyTx{Nonce: 1}),
		types.NewTx(&types.LegacyTx{Nonce: 2}),
	}

	// Check that parameters are correctly forwarded to the scheduler.
	any := gomock.Any()
	mockScheduler.EXPECT().Schedule(
		any,
		&scheduler.BlockInfo{
			Number:      idx.Block(latestBlock.Number) + 1,
			Time:        newBlockTime,
			GasLimit:    rules.Blocks.MaxBlockGas,
			MixHash:     common.Hash{}, // TODO: update as randao is integrated
			BaseFee:     uint256.Int{}, // TODO: implement
			BlobBaseFee: uint256.Int{}, // TODO: implement
		},
		nil,
		getEffectiveGasLimit(delta, rules.Economy.ShortGasPower.AllocPerSec),
	).Return(transactions)

	// Scheduling time should be monitored.
	durationMetric.EXPECT().Update(any).Do(func(duration time.Duration) {
		require.True(duration > 0)
	})

	// Run the payload creation.
	payload, err := createProposal(
		rules,
		state,
		latestBlock,
		newBlockTime,
		currentFrame,
		mockScheduler,
		nil,
		durationMetric,
		timeoutMetric,
	)

	require.NoError(err)
	require.Equal(state.lastSeenProposalTurn+1, payload.LastSeenProposalTurn)
	require.Equal(idx.Block(latestBlock.Number)+1, payload.LastSeenProposedBlock)
	require.Equal(currentFrame, payload.LastSeenProposalFrame)

	proposal := payload.Proposal
	require.Equal(idx.Block(latestBlock.Number)+1, proposal.Number)
	require.Equal(latestBlock.Hash(), proposal.ParentHash)
	require.Equal(newBlockTime, proposal.Time)
	require.Equal(transactions, proposal.Transactions)

	// TODO: check randao mix hash in proposal
}

func TestCreateProposal_InvalidBlockTime_ReturnsAnEmptyProposal(t *testing.T) {
	state := proposalState{
		lastSeenProposalTurn:  inter.Turn(5),
		lastSeenProposalFrame: idx.Frame(12),
		lastSeenProposedBlock: idx.Block(4),
	}
	latestBlock := inter.NewBlockBuilder().WithTime(1234).Build()
	for _, delta := range []time.Duration{-1 * time.Nanosecond, 0} {
		newTime := inter.Timestamp(1234) + inter.Timestamp(delta)
		payload, err := createProposal(
			opera.Rules{}, state, latestBlock, newTime, 0, nil, nil, nil, nil,
		)
		require.NoError(t, err)
		require.Equal(t, state.toPayload(), payload)
	}
}

func TestCreateProposal_IfSchedulerTimesOut_SignalTimeoutToMonitor(t *testing.T) {
	ctrl := gomock.NewController(t)
	mockScheduler := NewMocktxScheduler(ctrl)
	durationMetric := NewMocktimerMetric(ctrl)
	timeoutMetric := NewMockcounterMetric(ctrl)

	any := gomock.Any()
	mockScheduler.EXPECT().Schedule(any, any, any, any).Do(
		func(
			ctx context.Context, _ *scheduler.BlockInfo,
			_ scheduler.PrioritizedTransactions, _ uint64,
		) {
			deadline, ok := ctx.Deadline()
			require.True(t, ok, "scheduler call should have a deadline")
			for {
				delay := time.Until(deadline)
				if delay > 0 {
					<-time.After(delay)
				}
				if err := ctx.Err(); err != nil {
					require.ErrorIs(t, err, context.DeadlineExceeded)
					break
				}
			}
		})

	durationMetric.EXPECT().Update(any)
	timeoutMetric.EXPECT().Inc(int64(1))

	_, err := createProposal(
		opera.Rules{},
		proposalState{},
		inter.NewBlockBuilder().Build(),
		inter.Timestamp(1),
		0,
		mockScheduler,
		nil,
		durationMetric,
		timeoutMetric,
	)
	require.NoError(t, err)
}

func TestGetEffectiveGasLimit_IsProportionalToDelay(t *testing.T) {
	rates := []uint64{0, 1, 20, 1234, 10_000_000_000} // < gas/sec
	delay := []time.Duration{
		0, 1 * time.Nanosecond, 50 * time.Microsecond,
		100 * time.Millisecond, 1500 * time.Millisecond,
	}

	for _, rate := range rates {
		for _, d := range delay {
			got := getEffectiveGasLimit(d, rate)
			want := rate * uint64(d) / uint64(time.Second)
			require.Equal(t, want, got, "rate %d, delay %v", rate, d)
		}
	}
}

func TestGetEffectiveGasLimit_IsZeroForNegativeDelay(t *testing.T) {
	require.Equal(t, uint64(0), getEffectiveGasLimit(-1*time.Nanosecond, 100))
	require.Equal(t, uint64(0), getEffectiveGasLimit(-1*time.Second, 100))
	require.Equal(t, uint64(0), getEffectiveGasLimit(-1*time.Hour, 100))
}

func TestGetEffectiveGasLimit_IsCappedAtMaximumAccumulationTime(t *testing.T) {
	rate := uint64(100)
	maxAccumulationTime := maxAccumulationTime
	for _, d := range []time.Duration{
		maxAccumulationTime,
		maxAccumulationTime + 1*time.Nanosecond,
		maxAccumulationTime + 1*time.Second,
		maxAccumulationTime + 1*time.Hour,
	} {
		got := getEffectiveGasLimit(d, rate)
		want := getEffectiveGasLimit(maxAccumulationTime, rate)
		require.Equal(t, want, got, "delay %v", d)
	}
}

func TestTransactionPriorityAdapter_ForwardsCallToWrappedType(t *testing.T) {

	t.Run("Current", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		index := NewMocktransactionIndex(ctrl)

		tx := types.NewTx(&types.LegacyTx{Nonce: 1})
		index.EXPECT().Peek().Return(&txpool.LazyTransaction{Tx: tx}, nil)

		adapter := transactionPriorityAdapter{index}
		got := adapter.Current()
		require.Equal(t, tx, got)
	})

	t.Run("Current_Empty", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		index := NewMocktransactionIndex(ctrl)
		index.EXPECT().Peek().Return(nil, nil)
		adapter := transactionPriorityAdapter{index}
		got := adapter.Current()
		require.Nil(t, got)
	})

	t.Run("Accept", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		index := NewMocktransactionIndex(ctrl)
		index.EXPECT().Shift()
		adapter := transactionPriorityAdapter{index}
		adapter.Accept()
	})

	t.Run("Skip", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		index := NewMocktransactionIndex(ctrl)
		index.EXPECT().Pop()
		adapter := transactionPriorityAdapter{index}
		adapter.Skip()
	})
}
