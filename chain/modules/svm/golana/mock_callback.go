package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"
*/
import "C"

import (
	"fmt"
	"unsafe"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/cometbft/cometbft/libs/os"
	"github.com/cosmos/btcutil/base58"
	solanago "github.com/gagliardetto/solana-go"
)

var programBz []byte
var MockAccounts map[string]*types.Account

func init() {
	programBz, _ = os.ReadFile("example.so")
	MockAccounts = map[string]*types.Account{
		types.SystemProgramId.String(): {
			Pubkey:     types.SystemProgramId.Bytes(),
			Owner:      types.NativeProgramId.Bytes(),
			Lamports:   0,
			Data:       nil,
			Executable: true,
			RentEpoch:  0,
		},
		types.UpgradableLoaderProgramId.String(): {
			Pubkey:     types.UpgradableLoaderProgramId.Bytes(),
			Owner:      types.NativeProgramId.Bytes(),
			Lamports:   0,
			Data:       nil,
			Executable: true,
			RentEpoch:  0,
		},
	}
}

type MockCallbackContext struct {
	cbPtr *C.golana_tx_callback
	msgs  []*types.MsgTransaction
}

func NewMockCallbackContext() TxCallbackContextI {
	impl := &MockCallbackContext{}
	return impl
}

func (cb *MockCallbackContext) SetAllMsgs(msgs []*types.MsgTransaction) {
	cb.msgs = msgs
}

func (cb *MockCallbackContext) GetMsg(txId uint64) *types.MsgTransaction {
	if txId > uint64(len(cb.msgs)) {
		return nil
	}
	return cb.msgs[txId]
}

func (cb *MockCallbackContext) Execute(msg *types.MsgTransaction) (uint64, []string, error) {
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

func (cb *MockCallbackContext) ExecuteEndBlocker(nodeId int) (uint64, []string, error) {
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

func (cb *MockCallbackContext) GetAccount(pubkey []byte) TransactionAccount {
	account, exist := MockAccounts[base58.Encode(pubkey)]
	if !exist {
		account = &types.Account{
			Pubkey:     pubkey,
			Owner:      solanago.SystemProgramID[:],
			Lamports:   0,
			Data:       nil,
			Executable: false,
			RentEpoch:  0,
		}
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

func (cb *MockCallbackContext) SetAccount(account *types.Account) {
	AccountDebug(account)
}

func (cb *MockCallbackContext) Done() {
	cb.msgs = []*types.MsgTransaction{}
	callbackMap.Delete(uintptr(unsafe.Pointer(cb.cbPtr)))
	C.golana_tx_callback_free(cb.cbPtr)
}
