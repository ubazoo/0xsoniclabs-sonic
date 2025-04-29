package evmstore

import (
	"encoding/hex"
	"fmt"
	"github.com/0xsoniclabs/carmen/go/common/witness"
	"github.com/0xsoniclabs/sonic/inter/state"
	"github.com/ethereum/go-ethereum/common"
	geth_state "github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/trie/utils"
	"github.com/holiman/uint256"
	"os"
	"runtime/debug"
)

func WrapStateDbWithFileLogger(stateDb state.StateDB, blk uint64) (state.StateDB, error) {
	f, err := os.OpenFile(fmt.Sprintf("/var/data/data/logs/state_changes_%d.log", blk), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &loggingVmStateDb{
		stateDb,
		f,
		make(map[common.Address]struct{}),
	}, nil
}

type loggingVmStateDb struct {
	db             state.StateDB
	file           *os.File
	selfDestructed map[common.Address]struct{}
}

func (l *loggingVmStateDb) EndTransaction() {
	l.writeLog("EndTransaction")
	l.db.EndTransaction()
}

func (l *loggingVmStateDb) AccessEvents() *geth_state.AccessEvents {
	res := l.db.AccessEvents()
	l.writeLog("AccessEvents, %v", res)
	return res
}

func (l *loggingVmStateDb) Error() error {
	res := l.db.Error()
	l.writeLog("Error, %v", res)
	return res
}

func (l *loggingVmStateDb) TxIndex() int {
	res := l.db.TxIndex()
	l.writeLog("TxIndex, %v", res)
	return res
}

func (l *loggingVmStateDb) GetProof(addr common.Address, keys []common.Hash) (witness.Proof, error) {
	res, err := l.db.GetProof(addr, keys)
	l.writeLog("GetProof, %v, %v, %v, %v", addr, keys, res, err)
	return res, err
}

func (l *loggingVmStateDb) SetBalance(addr common.Address, amount *uint256.Int) {
	l.db.SetBalance(addr, amount)
	l.writeLog("SetBalance, %v, %v", addr, amount)
}

func (l *loggingVmStateDb) SetStorage(addr common.Address, storage map[common.Hash]common.Hash) {
	l.db.SetStorage(addr, storage)
	l.writeLog("SetBalance, %v, %v", addr, storage)
}

func (l *loggingVmStateDb) Copy() state.StateDB {
	l.writeLog("Copy")
	return l.db.Copy()
}

func (l *loggingVmStateDb) GetStateHash() common.Hash {
	res := l.db.GetStateHash()
	l.writeLog("GetStateHash, %v", res)
	return res
}

func (l *loggingVmStateDb) BeginBlock(number uint64) {
	l.writeLog("BeginBlock, %v", number)
	l.db.BeginBlock(number)
}

func (l *loggingVmStateDb) EndBlock(number uint64) {
	l.writeLog("EndBlock, %v", number)
	l.db.BeginBlock(number)
}

func (l *loggingVmStateDb) Release() {
	l.writeLog("Release")
	l.file.Close()
	l.db.Release()
}

func (l *loggingVmStateDb) CreateAccount(addr common.Address) {
	l.db.CreateAccount(addr)
	l.writeLog("CreateAccount, %v", addr)
}

func (l *loggingVmStateDb) Exist(addr common.Address) bool {
	res := l.db.Exist(addr)
	l.writeLog("Exist, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) Empty(addr common.Address) bool {
	res := l.db.Empty(addr)
	l.writeLog("Empty, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) SelfDestruct(addr common.Address) uint256.Int {
	res := l.db.SelfDestruct(addr)
	l.writeLog("SelfDestruct, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) HasSelfDestructed(addr common.Address) bool {
	res := l.db.HasSelfDestructed(addr)
	l.writeLog("HasSelfDestructed, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) GetBalance(addr common.Address) *uint256.Int {
	res := l.db.GetBalance(addr)
	l.writeLog("GetBalance, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) AddBalance(addr common.Address, value *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	res := l.db.AddBalance(addr, value, reason)
	l.writeLog("AddBalance, %v, %v, %v, %v, %v", addr, value, l.db.GetBalance(addr), reason, res)
	return res
}

func (l *loggingVmStateDb) SubBalance(addr common.Address, value *uint256.Int, reason tracing.BalanceChangeReason) uint256.Int {
	res := l.db.SubBalance(addr, value, reason)
	l.writeLog("SubBalance, %v, %v, %v, %v, %v", addr, value, l.db.GetBalance(addr), reason, res)
	return res
}

func (l *loggingVmStateDb) GetNonce(addr common.Address) uint64 {
	res := l.db.GetNonce(addr)
	l.writeLog("GetNonce, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) SetNonce(addr common.Address, value uint64, reason tracing.NonceChangeReason) {
	l.db.SetNonce(addr, value, reason)
	l.writeLog("SetNonce, %v, %v, %v", addr, value, reason)
}

func (l *loggingVmStateDb) GetCommittedState(addr common.Address, key common.Hash) common.Hash {
	res := l.db.GetCommittedState(addr, key)
	l.writeLog("GetCommittedState, %v, %v, %v", addr, key, res)
	return res
}

func (l *loggingVmStateDb) GetState(addr common.Address, key common.Hash) common.Hash {
	res := l.db.GetState(addr, key)
	l.writeLog("GetState, %v, %v, %v", addr, key, res)
	return res
}

func (l *loggingVmStateDb) SetState(addr common.Address, key common.Hash, value common.Hash) common.Hash {
	res := l.db.SetState(addr, key, value)
	l.writeLog("SetState, %v, %v, %v, %v", addr, key, value, res)
	return res
}

func (l *loggingVmStateDb) SetTransientState(addr common.Address, key common.Hash, value common.Hash) {
	l.writeLog("SetTransientState, %v, %v, %v", addr, key, value)
	l.db.SetTransientState(addr, key, value)
}

func (l *loggingVmStateDb) GetTransientState(addr common.Address, key common.Hash) common.Hash {
	value := l.db.GetTransientState(addr, key)
	l.writeLog("GetTransientState, %v, %v, %v", addr, key, value)
	return value
}

func (l *loggingVmStateDb) GetCode(addr common.Address) []byte {
	res := l.db.GetCode(addr)
	l.writeLog("GetCode, %v, %v", addr, hex.EncodeToString(res))
	return res
}

func (l *loggingVmStateDb) GetCodeSize(addr common.Address) int {
	res := l.db.GetCodeSize(addr)
	l.writeLog("GetCodeSize, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) GetCodeHash(addr common.Address) common.Hash {
	res := l.db.GetCodeHash(addr)
	l.writeLog("GetCodeHash, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) SetCode(addr common.Address, code []byte) []byte {
	res := l.db.SetCode(addr, code)
	l.writeLog("SetCode, %v, %v, %v", addr, code, res)
	return res
}

func (l *loggingVmStateDb) Snapshot() int {
	res := l.db.Snapshot()
	l.writeLog("Snapshot, %v", res)
	return res
}

func (l *loggingVmStateDb) RevertToSnapshot(id int) {
	l.db.RevertToSnapshot(id)
	l.writeLog("RevertToSnapshot, %v", id)
}

func (l *loggingVmStateDb) Finalise(deleteEmptyObjects bool) {
	l.writeLog("Finalise, %v", deleteEmptyObjects)
	l.db.Finalise(deleteEmptyObjects)
}

func (l *loggingVmStateDb) AddRefund(amount uint64) {
	l.db.AddRefund(amount)
	l.writeLog("AddRefund, %v, %v", amount, l.db.GetRefund())
}

func (l *loggingVmStateDb) SubRefund(amount uint64) {
	l.db.SubRefund(amount)
	l.writeLog("SubRefund, %v, %v", amount, l.db.GetRefund())
}

func (l *loggingVmStateDb) GetRefund() uint64 {
	res := l.db.GetRefund()
	l.writeLog("GetRefund, %v", res)
	return res
}

func (l *loggingVmStateDb) Prepare(rules params.Rules, sender, coinbase common.Address, dest *common.Address, precompiles []common.Address, txAccesses types.AccessList) {
	l.writeLog("Prepare, %v, %v, %v, %v", sender, dest, precompiles, txAccesses)
	l.db.Prepare(rules, sender, coinbase, dest, precompiles, txAccesses)
}

func (l *loggingVmStateDb) AddressInAccessList(addr common.Address) bool {
	res := l.db.AddressInAccessList(addr)
	l.writeLog("AddressInAccessList, %v, %v", addr, res)
	return res
}

func (l *loggingVmStateDb) SlotInAccessList(addr common.Address, slot common.Hash) (addressOk bool, slotOk bool) {
	a, b := l.db.SlotInAccessList(addr, slot)
	l.writeLog("SlotInAccessList, %v, %v, %v, %v", addr, slot, a, b)
	return a, b
}

func (l *loggingVmStateDb) AddAddressToAccessList(addr common.Address) {
	l.writeLog("AddAddressToAccessList, %v", addr)
	l.db.AddAddressToAccessList(addr)
}

func (l *loggingVmStateDb) AddSlotToAccessList(addr common.Address, slot common.Hash) {
	l.writeLog("AddSlotToAccessList, %v, %v", addr, slot)
	l.db.AddSlotToAccessList(addr, slot)
}

func (l *loggingVmStateDb) AddLog(entry *types.Log) {
	l.writeLog("AddLog, %v", entry)
	l.db.AddLog(entry)
}

func (l *loggingVmStateDb) GetLogs(hash common.Hash, blockHash common.Hash) []*types.Log {
	res := l.db.GetLogs(hash, blockHash)
	l.writeLog("GetLogs, %v, %v, %v, %v", hash, blockHash, res)
	return res
}

// PointCache returns the point cache used in computations.
func (l *loggingVmStateDb) PointCache() *utils.PointCache {
	res := l.db.PointCache()
	l.writeLog("PointCache, %v", res)
	return res
}

// Witness retrieves the current state witness.
func (l *loggingVmStateDb) Witness() *stateless.Witness {
	res := l.db.Witness()
	l.writeLog("Witness, %v", res)
	return res
}

func (l *loggingVmStateDb) SetTxContext(thash common.Hash, ti int) {
	l.db.SetTxContext(thash, ti)
	l.writeLog("SetTxContext, %v, %v\nSTACK TRACE: %s\n", thash, ti, string(debug.Stack()))
}

func (l *loggingVmStateDb) AddPreimage(hash common.Hash, data []byte) {
	l.db.AddPreimage(hash, data)
	l.writeLog("AddPreimage, %v, %v", hash, data)
}

func (l *loggingVmStateDb) CreateContract(addr common.Address) {
	l.writeLog("CreateContract, %v", addr)
	l.db.CreateContract(addr)
}

func (l *loggingVmStateDb) SelfDestruct6780(addr common.Address) (uint256.Int, bool) {
	balance, success := l.db.SelfDestruct6780(addr)
	l.writeLog("SelfDestruct6780, %v, %v, %v", addr, balance, success)
	return balance, success
}

func (l *loggingVmStateDb) GetStorageRoot(addr common.Address) common.Hash {
	res := l.db.GetStorageRoot(addr)
	l.writeLog("GetStorageRoot, %v, %v", res, addr)
	return res
}

func (l *loggingVmStateDb) writeLog(format string, a ...any) {
	str := fmt.Sprintf(format, a...)
	_, err := l.file.WriteString(str + "\n")
	if err != nil {
		log.Error("cannot write into db-log-file;", "error", err)
	}
}
