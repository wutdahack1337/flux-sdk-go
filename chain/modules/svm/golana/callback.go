package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"

extern golana_compute_budget *getComputeBudget(void *caller, uint64_t tx_id);

extern golana_pubkeys *getPubkeys(void *caller, uint64_t tx_id);

extern uint64_t getIxLen(void *caller, uint64_t tx_id);

extern golana_ix_info *getIxInfo(void *caller, uint64_t tx_id, uint64_t ix_id);

extern golana_transaction_account *getAccountSharedData(void *caller, uint8_t *pubkey);

extern bool setAccountSharedData(void *caller, golana_transaction_account *account);

static golana_tx_callback *golana_tx_callback_wrapper_new() {
	return golana_tx_callback_create(
		getComputeBudget,
		getPubkeys,
		getIxLen,
		getIxInfo,
		getAccountSharedData,
		setAccountSharedData
	);
}

*/
import "C"

import (
	"context"
	"fmt"
	"sync"
	"unsafe"

	"github.com/cosmos/btcutil/base58"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var callbackMap = new(sync.Map)

// expose golang method so multiple impls of callback interface can access
func TxCallbackWrapperNew() *C.golana_tx_callback {
	return C.golana_tx_callback_wrapper_new()
}

type VmKeeper interface {
	GetDefaultAccount(accBz []byte) *types.Account
	KVGetAccount(ctx context.Context, accAddr []byte) (*types.Account, bool)
	KVSetAccount(ctx context.Context, account *types.Account)
}

type TxCallbackContextI interface {
	SetAllMsgs(msgs []*types.MsgTransaction)
	GetMsg(txId uint64) *types.MsgTransaction
	Execute(msg *types.MsgTransaction) (uint64, []string, error)
	ExecuteEndBlocker(nodeId int) (uint64, []string, error)
	GetAccount(pk []byte) TransactionAccount
	SetAccount(acc *types.Account)
	Done()
}

var _ TxCallbackContextI = &TxCallbackContext{}

// implement a default callback set
type TxCallbackContext struct {
	ctx      sdk.Context
	vmKeeper VmKeeper
	msgs     []*types.MsgTransaction
	cbPtr    *C.golana_tx_callback
}

func NewTxCallbackContext(ctx sdk.Context, keeper VmKeeper) TxCallbackContextI {
	impl := &TxCallbackContext{
		ctx:      ctx,
		vmKeeper: keeper,
		msgs:     []*types.MsgTransaction{},
	}
	return impl
}

func (cb *TxCallbackContext) SetAllMsgs(msgs []*types.MsgTransaction) {
	cb.msgs = msgs
}

func (cb *TxCallbackContext) GetMsg(txId uint64) *types.MsgTransaction {
	if txId > uint64(len(cb.msgs)) {
		return nil
	}
	return cb.msgs[txId]
}

func (cb *TxCallbackContext) Execute(msg *types.MsgTransaction) (uint64, []string, error) {
	cb.cbPtr = TxCallbackWrapperNew()
	callbackMap.Store(uintptr(unsafe.Pointer(cb.cbPtr)), cb)

	// only execute 1 msg
	cb.msgs = []*types.MsgTransaction{msg}

	// execute all instructions in a msg
	totalUnitConsumed := C.uint64_t(0)
	result := C.golana_execute(cb.cbPtr, C.uint64_t(0), &totalUnitConsumed)

	// unpack log
	length := int(C.golana_result_log_len(result))
	logsPtr := (*[1 << 30]*C.char)(unsafe.Pointer(C.golana_result_log_ptr(result)))[:length:length]
	logs := make([]string, length)
	for i, ptr := range logsPtr {
		logs[i] = C.GoString(ptr)
	}

	if C.golana_result_error(result) != nil {
		err := C.GoString(C.golana_result_error(result))
		return 0, logs, fmt.Errorf(err)
	}

	// free result
	C.golana_result_free(result)

	return uint64(totalUnitConsumed), logs, nil
}

func (cb *TxCallbackContext) ExecuteEndBlocker(nodeId int) (uint64, []string, error) {
	ptr := TxCallbackWrapperNew()
	callbackMap.Store(uintptr(unsafe.Pointer(ptr)), cb)

	unitConsumed := C.uint64_t(0)
	result := C.golana_execute(ptr, C.uint64_t(nodeId), &unitConsumed)

	// unpack log
	length := int(C.golana_result_log_len(result))
	logsPtr := (*[1 << 30]*C.char)(unsafe.Pointer(C.golana_result_log_ptr(result)))[:length:length]
	logs := make([]string, length)
	for i, ptr := range logsPtr {
		logs[i] = C.GoString(ptr)
	}

	if C.golana_result_error(result) != nil {
		err := C.GoString(C.golana_result_error(result))
		return 0, logs, fmt.Errorf(err)
	}

	// free result
	C.golana_result_free(result)

	return uint64(unitConsumed), logs, nil
}

func (cb *TxCallbackContext) GetAccount(pubkey []byte) TransactionAccount {
	// try to get from kvstore
	account, exist := cb.vmKeeper.KVGetAccount(cb.ctx, pubkey)
	if !exist {
		account = cb.vmKeeper.GetDefaultAccount(pubkey)
	}
	return NewTransactionAccount(
		account.Pubkey,
		account.Owner,
		account.Lamports,
		account.Data,
		account.Executable,
		account.RentEpoch,
	)
}

func (cb *TxCallbackContext) SetAccount(account *types.Account) {
	/*
		svm allows account creation with
		0 lamports / 0 data => rent exempted
		0 lamports / n data => not rent exempted => will be reclaimed right away by solana
	*/
	// we can stay in line with solana behavior here by simply skipping persisting data when lamports < rent exemption amount
	if account.Lamports < types.GetRentExemptLamportAmount(uint64(len(account.Data))) {
		return
	}
	cb.vmKeeper.KVSetAccount(cb.ctx, account)
}

func (cb *TxCallbackContext) Done() {
	cb.msgs = []*types.MsgTransaction{}
	callbackMap.Delete(uintptr(unsafe.Pointer(cb.cbPtr)))
	C.golana_tx_callback_free(cb.cbPtr)
}

//export getComputeBudget
func getComputeBudget(caller *C.void, tx_id C.uint64_t) *C.golana_compute_budget {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return nil
	}

	msg := wrapper.(TxCallbackContextI).GetMsg(uint64(tx_id))
	computeBudget := NewComputeBudget(msg.ComputeBudget, types.DefaultMaxInstructionTraceLength, 8)
	return computeBudget.computeBudget
}

//export getPubkeys
func getPubkeys(caller *C.void, tx_id C.uint64_t) *C.golana_pubkeys {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return nil
	}
	msg := wrapper.(TxCallbackContextI).GetMsg(uint64(tx_id))
	pubkeys := [][]byte{}
	for _, acc := range msg.Accounts {
		pubkey := base58.Decode(acc)
		pubkeys = append(pubkeys, pubkey)
	}
	svmPubkeys := NewPubkeys(pubkeys)
	return svmPubkeys.pubkeys
}

//export getIxLen
func getIxLen(caller *C.void, tx_id C.uint64_t) C.uint64_t {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return 0
	}
	msg := wrapper.(TxCallbackContextI).GetMsg(uint64(tx_id))
	return C.uint64_t(len(msg.Instructions))
}

//export getIxInfo
func getIxInfo(caller *C.void, tx_id C.uint64_t, ix_id C.uint64_t) *C.golana_ix_info {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return nil
	}
	msg := wrapper.(TxCallbackContextI).GetMsg(uint64(tx_id))
	ix := msg.Instructions[ix_id]
	ixAccounts := []InstructionAccount{}
	for _, acc := range ix.Accounts {
		ixAccounts = append(ixAccounts, NewInstructionAccount(
			uint16(acc.IdIndex),
			uint16(acc.CallerIndex),
			uint16(acc.CalleeIndex),
			acc.IsSigner,
			acc.IsWritable,
		))
	}
	ixPrograms := []uint16{}
	for _, id := range ix.ProgramIndex {
		ixPrograms = append(ixPrograms, uint16(id))
	}
	ixInfo := NewIxInfo(ixAccounts, ixPrograms, ix.Data)
	return ixInfo.ixInfo
}

//export getAccountSharedData
func getAccountSharedData(caller *C.void, pubkey *C.uint8_t) *C.golana_transaction_account {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return nil
	}

	goPubkey := C.GoBytes(unsafe.Pointer(pubkey), 32)
	sharedData := wrapper.(TxCallbackContextI).GetAccount(goPubkey)
	return sharedData.account
}

//export setAccountSharedData
func setAccountSharedData(caller *C.void, account *C.golana_transaction_account) C.bool {
	id := uintptr(unsafe.Pointer(caller))
	wrapper, exist := callbackMap.Load(id)
	if !exist || wrapper == nil {
		return C.bool(false)
	}
	acc := toGoAccount(account)
	wrapper.(TxCallbackContextI).SetAccount(acc)
	return C.bool(true)
}
