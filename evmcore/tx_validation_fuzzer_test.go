package evmcore

import (
	"fmt"
	"iter"
	"math/big"
	"strings"
	"testing"

	"github.com/0xsoniclabs/sonic/inter/state"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testBerlin   = 0x01
	testLondon   = 0x02
	testShanghai = 0x03
	testCancun   = 0x04
	testPrague   = 0x05
)

// FuzzValidateTransaction fuzzes the validateTx function with randomly generated transactions.
func FuzzValidateTransaction(f *testing.F) {

	// Seed corpus with a few valid-looking values
	f.Add(uint8(0), uint64(1), uint64(42_000), int64(1_000), int64(500), int64(0),
		[]byte("hi"), false,
		int64(1_000_000_000), int8(1), uint(0), uint(0),
	)
	f.Add(uint8(1), uint64(1), uint64(42_000), int64(1_000), int64(500), int64(0),
		[]byte("123456789123456789123456789123456789123456789123456789"), false,
		int64(123_456_789), int8(2), uint(1), uint(0),
	)
	f.Add(uint8(2), uint64(1), uint64(42_000), int64(1_000), int64(500), int64(0),
		[]byte(""), false,
		int64(1_000_000_000), int8(3), uint(2), uint(0),
	)
	f.Add(uint8(3), uint64(1), uint64(42_000), int64(1_000), int64(500), int64(0),
		[]byte("some"), false,
		int64(1_000_000_000), int8(4), uint(3), uint(0),
	)
	f.Add(uint8(4), uint64(1), uint64(42_000), int64(1_000), int64(500), int64(0),
		[]byte("code"), false,
		int64(1_000_000_000), int8(5), uint(3), uint(1),
	)

	f.Fuzz(func(t *testing.T,
		// transaction values
		txType uint8, nonce, gas uint64, feeCap, tip, value int64,
		data []byte, isCreate bool,
		// block context
		blockNum int64, revision int8,
		// because the number of elements in access list and authorization list
		// affects intrinsic gas cost, these values are fuzzed,
		// but the actual values are not relevant for this test.
		accessListSize, authListSize uint,
	) {

		if txType > types.SetCodeTxType {
			// Skip invalid transaction types
			return
		}
		if revision > testPrague {
			// Skip invalid revisions
			return
		}
		if isCreate && (txType == types.BlobTxType || txType == types.SetCodeTxType) {
			// Skip invalid transaction types for contract creation
			return
		}

		// a full persistent state is not need. ValidateTx needs to see the same state as the processor.
		ctxt := gomock.NewController(t)

		chainId := big.NewInt(84)

		tx := makeTxOfType(txType, nonce, gas, feeCap, tip, data, value,
			isCreate, chainId, accessListSize, authListSize)
		from, signedTx := signTxForTestWithChainId(t, tx, chainId)

		// We intentionally avoid fuzzing these specific values to explicitly
		// test the key edge cases: under, exact, and over.
		// The magnitude of the difference doesn't matter;
		// what's important is whether the value is below, equal to, or above
		// the expected threshold.
		fuzzedCombo := combo{
			cost:    signedTx.Cost(),
			nonce:   nonce,
			gas:     gas,
			baseFee: feeCap,
			tip:     tip,
		}
		for combo := range generateOverUnderCombinations(fuzzedCombo) {
			testCase := fmt.Sprintf("cost: %d, nonce: %d, gas: %d, feeCap: %d, tip: %d",
				combo.cost, combo.nonce, combo.gas, combo.baseFee, combo.tip)
			t.Run(testCase, func(t *testing.T) {

				stateBalance := combo.cost
				stateNonce := combo.nonce
				validateGas := combo.gas
				validateBaseFee := combo.baseFee
				validateMinTip := combo.tip

				// for combo := range combos {
				state := state.NewMockStateDB(ctxt)
				state.EXPECT().GetBalance(from).Return(uint256.MustFromBig(stateBalance)).AnyTimes()
				state.EXPECT().GetNonce(from).Return(stateNonce).AnyTimes()
				if txType == types.SetCodeTxType {
					state.EXPECT().GetCode(gomock.Any()).Return([]byte("some code")).AnyTimes()
				} else {
					// This must return empty because externally owned accounts cannot have
					// code prior to the Prague revision.
					state.EXPECT().GetCode(gomock.Any()).Return([]byte{}).AnyTimes()
				}
				stateExpectCalls(state)

				opt := getTestTransactionsOptionFromRevision(revision, chainId,
					validateGas, validateBaseFee, validateMinTip)
				opt.currentState = state
				opt.currentMaxGas = uint64(validateBaseFee)

				// Validate the transaction
				validateErr := validateTx(signedTx, opt)

				// create evm to check validateTx is consistent with processor.
				evm := makeTestEvm(blockNum, validateBaseFee, validateGas, state, revision, chainId)

				msg, err := TxAsMessage(
					signedTx,
					types.NewPragueSigner(chainId), // use same sender as signTxForTest
					evm.Context.BaseFee)
				require.NoError(t, err)

				gp := new(core.GasPool).AddGas(uint64(validateBaseFee))
				var usedGas uint64
				_, _, _, processorError := applyTransaction(msg, gp, state, big.NewInt(blockNum),
					signedTx, &usedGas, evm, nil)

				if validateErr == nil {
					if processorError != validateErr &&
						// if the nonce is too high this is also acceptable for the validateTx
						!strings.Contains(processorError.Error(), "nonce too high") {
						t.Fatalf("\nvalidateTx: %v - applyTx: %v\n", validateErr, processorError)
					}
				}
			})
		}
	})
}

type combo struct {
	cost    *big.Int
	nonce   uint64
	gas     uint64
	baseFee int64
	tip     int64
}

func generateOverUnderCombinations(c combo) iter.Seq[combo] {
	// Generate all combinations of the given values
	underCost := new(big.Int).Sub(c.cost, big.NewInt(1))
	overCost := new(big.Int).Add(c.cost, big.NewInt(1))

	costs := []*big.Int{underCost, c.cost, overCost}
	nonces := []uint64{c.nonce - 1, c.nonce, c.nonce + 1}
	gasValues := []uint64{c.gas - 1, c.gas, c.gas + 1}
	baseFees := []int64{c.baseFee - 1, c.baseFee, c.baseFee + 1}
	tips := []int64{c.tip - 1, c.tip, c.tip + 1}

	return func(yield func(c1 combo) bool) {
		for _, balance := range costs {
			for _, nonce := range nonces {
				for _, gas := range gasValues {
					for _, baseFee := range baseFees {
						for _, tip := range tips {
							if !yield(combo{
								cost:    balance,
								nonce:   nonce,
								gas:     gas,
								baseFee: baseFee,
								tip:     tip,
							}) {
								return
							}
						}
					}
				}
			}
		}
	}
}

func TestGenerateOverUnderCombinations_GeneratesAllCombinations(t *testing.T) {

	// Define the input values
	c := combo{
		cost:    big.NewInt(100),
		nonce:   1,
		gas:     200,
		baseFee: 50,
		tip:     10,
	}

	repeated := false
	seen := make(map[combo]struct{})
	comboSet := generateOverUnderCombinations(c)

	for combo := range comboSet {
		if _, exists := seen[combo]; exists {
			repeated = true
			break
		} else {
			seen[combo] = struct{}{}
		}
	}

	require.Len(t, seen, 243)
	require.False(t, repeated, "Duplicate combinations found")

}

func makeTxOfType(txType uint8, nonce, gas uint64, feeCap, tip int64,
	data []byte, value int64, isCreate bool, chainId *big.Int,
	accessListSize, authListSize uint) types.TxData {

	feeCapBig := big.NewInt(feeCap)
	tipBig := big.NewInt(tip)

	to := common.Address{0x42}
	toPtr := &to
	if isCreate {
		// Set to nil
		toPtr = nil
	}

	accessList := make([]types.AccessTuple, accessListSize)
	for i := range accessListSize {
		accessList[i] = types.AccessTuple{}
	}
	authList := make([]types.SetCodeAuthorization, authListSize)
	for j := range authListSize {
		authList[j] = types.SetCodeAuthorization{}
	}

	var tx types.TxData
	switch txType {
	case types.LegacyTxType:
		tx = &types.LegacyTx{
			Nonce:    nonce,
			Gas:      gas,
			GasPrice: feeCapBig,
			To:       toPtr,
			Value:    big.NewInt(value),
			Data:     data,
		}
	case types.AccessListTxType:
		tx = &types.AccessListTx{
			ChainID:    chainId,
			Nonce:      nonce,
			Gas:        gas,
			GasPrice:   feeCapBig,
			To:         toPtr,
			Value:      big.NewInt(value),
			Data:       data,
			AccessList: accessList,
		}
	case types.DynamicFeeTxType:
		tx = &types.DynamicFeeTx{
			ChainID:    chainId,
			Nonce:      nonce,
			Gas:        gas,
			GasFeeCap:  feeCapBig,
			GasTipCap:  tipBig,
			To:         toPtr,
			Value:      big.NewInt(value),
			Data:       data,
			AccessList: accessList,
		}
	case types.BlobTxType:
		tx = &types.BlobTx{
			ChainID:    uint256.MustFromBig(chainId),
			Nonce:      nonce,
			Gas:        gas,
			GasFeeCap:  uint256.MustFromBig(feeCapBig),
			GasTipCap:  uint256.MustFromBig(tipBig),
			To:         to, // cannot be create
			Value:      uint256.NewInt(uint64(value)),
			Data:       data,
			AccessList: accessList,
		}
	case types.SetCodeTxType:
		tx = &types.SetCodeTx{
			ChainID:    uint256.MustFromBig(chainId),
			Gas:        gas,
			GasFeeCap:  uint256.MustFromBig(feeCapBig),
			GasTipCap:  uint256.MustFromBig(tipBig),
			To:         to, // cannot be create
			Value:      uint256.NewInt(uint64(value)),
			Data:       data,
			AccessList: accessList,
			AuthList:   authList,
		}
	}
	return tx
}

func stateExpectCalls(state *state.MockStateDB) {
	// expected calls to the state
	any := gomock.Any()
	state.EXPECT().Snapshot().AnyTimes()
	state.EXPECT().RevertToSnapshot(any).AnyTimes()

	// All accounts are preloaded in the state to avoid unintended early
	// exits or implicit account creation.
	state.EXPECT().Exist(any).Return(true).AnyTimes()

	state.EXPECT().CreateAccount(any).AnyTimes()
	state.EXPECT().CreateContract(any).AnyTimes()
	state.EXPECT().EndTransaction().AnyTimes()
	state.EXPECT().TxIndex().Return(4).AnyTimes()

	state.EXPECT().GetCodeHash(any).Return(types.EmptyCodeHash).AnyTimes()
	state.EXPECT().GetCodeSize(any).Return(0).AnyTimes()
	state.EXPECT().GetLogs(any, any).Return([]*types.Log{}).AnyTimes()
	state.EXPECT().GetNonce(any).Return(uint64(1)).AnyTimes()
	state.EXPECT().GetRefund().Return(uint64(0)).AnyTimes()
	state.EXPECT().GetState(any, any).Return(common.Hash{}).AnyTimes()
	state.EXPECT().GetStorageRoot(any).Return(types.EmptyRootHash).AnyTimes()

	state.EXPECT().HasSelfDestructed(any).Return(false).AnyTimes()
	state.EXPECT().SelfDestruct(any).AnyTimes()

	state.EXPECT().AddBalance(any, any, any).AnyTimes()
	state.EXPECT().AddLog(any).AnyTimes()
	state.EXPECT().AddRefund(any).AnyTimes()

	state.EXPECT().SetCode(any, any).Return([]byte{}).AnyTimes()
	state.EXPECT().SetNonce(any, any, any).AnyTimes()
	state.EXPECT().SetState(any, any, any).Return(common.Hash{}).AnyTimes()

	state.EXPECT().AddAddressToAccessList(any).AnyTimes()
	state.EXPECT().Prepare(any, any, any, any, any, any).AnyTimes()
	state.EXPECT().SubBalance(any, any, any).AnyTimes()
	state.EXPECT().Witness().AnyTimes()
}

func makeTestEvm(blockNum, basefee int64, evmGasPrice uint64, state vm.StateDB, revision int8, chainId *big.Int) *vm.EVM {

	chainConfig := &params.ChainConfig{
		ChainID:             chainId,
		EIP150Block:         new(big.Int),
		EIP155Block:         new(big.Int),
		EIP158Block:         new(big.Int),
		ByzantiumBlock:      new(big.Int),
		ConstantinopleBlock: new(big.Int),
		PetersburgBlock:     new(big.Int),
		IstanbulBlock:       new(big.Int),
		MuirGlacierBlock:    new(big.Int),
	}

	u64One := uint64(1)
	blockTime := uint64(0)

	switch revision {
	case testPrague:
		chainConfig.PragueTime = &u64One
		fallthrough
	case testCancun:
		chainConfig.CancunTime = &u64One
		fallthrough
	case testShanghai:
		chainConfig.ShanghaiTime = &u64One
		blockTime = 1
		fallthrough
	case testLondon:
		chainConfig.LondonBlock = common.Big0
		fallthrough
	case testBerlin:
		chainConfig.BerlinBlock = common.Big0
		fallthrough
	default:
		chainConfig.IstanbulBlock = common.Big0
	}
	random := common.Hash([32]byte{0x42})
	evm := vm.NewEVM(
		vm.BlockContext{
			BlockNumber: big.NewInt(blockNum),
			Difficulty:  big.NewInt(1),
			BaseFee:     big.NewInt(basefee),
			BlobBaseFee: big.NewInt(0),
			Random:      &random,
			Time:        blockTime,

			Transfer:    vm.TransferFunc(func(sd vm.StateDB, a1, a2 common.Address, i *uint256.Int) {}),
			CanTransfer: vm.CanTransferFunc(func(sd vm.StateDB, a1 common.Address, i *uint256.Int) bool { return true }),
			GetHash:     func(i uint64) common.Hash { return common.Hash{} },
		},
		state,
		chainConfig,
		vm.Config{},
	)
	evm.GasPrice = big.NewInt(int64(evmGasPrice))
	return evm
}

// signTxForTest generates a new key, signs the transaction with it, and returns
// the signer, address, and signed transaction.
func signTxForTestWithChainId(t *testing.T, tx types.TxData, chainId *big.Int) (common.Address, *types.Transaction) {
	key, err := crypto.GenerateKey()
	address := crypto.PubkeyToAddress(key.PublicKey)
	require.NoError(t, err)
	signer := types.NewPragueSigner(chainId)
	signedTx, err := types.SignTx(types.NewTx(tx), signer, key)
	require.NoError(t, err)
	return address, signedTx
}

func getTestTransactionsOptionFromRevision(revision int8, chainId *big.Int,
	maxGas uint64, BaseFee, MinTip int64) validationOptions {
	opt := validationOptions{
		currentMaxGas:  maxGas,
		currentBaseFee: big.NewInt(BaseFee),
		minTip:         big.NewInt(MinTip),
		isLocal:        true,
		signer:         types.NewPragueSigner(chainId),
	}

	switch revision {
	case testPrague:
		opt.eip7702 = true
		opt.eip7623 = true
		fallthrough
	case testCancun:
		opt.eip4844 = true
		fallthrough
	case testShanghai:
		opt.shanghai = true
		fallthrough
	case testLondon:
		opt.eip1559 = true
		fallthrough
	case testBerlin:
		opt.eip2718 = true
		fallthrough
	default:
		opt.istanbul = true
	}

	return opt
}
