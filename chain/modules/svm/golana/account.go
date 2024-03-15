package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"
*/
import "C"
import (
	"encoding/hex"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/cosmos/btcutil/base58"
	goerrors "github.com/pkg/errors"
	"unsafe"
)

type TransactionAccount struct {
	Account *C.c_transaction_account
}

func NewTransactionAccount(
	pubkey []byte,
	owner []byte,
	lamports uint64,
	data []byte,
	executable bool,
	rentEpoch uint64,
) *TransactionAccount {
	pubkeyPtr := C.CBytes(pubkey)
	ownerPtr := C.CBytes(owner)
	dataPtr := C.CBytes(data)
	return &TransactionAccount{
		C.transaction_account_create(
			(*C.uchar)(pubkeyPtr),
			(*C.uchar)(ownerPtr),
			C.ulonglong(lamports),
			(*C.uchar)(dataPtr), C.ulong(len(data)),
			C.bool(executable),
			C.ulonglong(rentEpoch),
		),
	}
}

func (c *TransactionAccount) Free() error {
	ptr := c.Account
	if ptr == nil {
		return goerrors.New("nil pointer err")
	}

	C.transaction_account_free(ptr)
	return nil
}

type InstructionAccount struct {
	Account *C.c_instruction_account
}

func NewInstructionAccount(
	transactionIndex uint16,
	callerIndex uint16,
	calleeIndex uint16,
	isSigner bool,
	isWritable bool) *InstructionAccount {
	return &InstructionAccount{
		C.instruction_account_create(
			C.ushort(transactionIndex),
			C.ushort(callerIndex),
			C.ushort(calleeIndex),
			C.bool(isSigner),
			C.bool(isWritable),
		),
	}
}

func (c *InstructionAccount) Free() {
	C.instruction_account_free(c.Account)
}

func AccountDebug(a *types.Account) {
	fmt.Println("---\npubkey:", base58.Encode(a.Pubkey))
	fmt.Println("owner:", base58.Encode(a.Owner))
	fmt.Println("lamports:", a.Lamports)
	fmt.Println("data:", hex.EncodeToString(a.Data))
	fmt.Println("isExecutable:", a.Executable)
	fmt.Println("rentEpoch:", a.RentEpoch)
}

func toGoAccount(accountPtr *C.c_transaction_account) *types.Account {
	dataVec := C.transaction_account_get_data(accountPtr)
	return &types.Account{
		Pubkey:     C.GoBytes(unsafe.Pointer(C.transaction_account_get_pubkey(accountPtr)), 32),
		Owner:      C.GoBytes(unsafe.Pointer(C.transaction_account_get_owner(accountPtr)), 32),
		Lamports:   uint64(C.transaction_account_get_lamports(accountPtr)),
		Data:       C.GoBytes(unsafe.Pointer(dataVec.data), C.int(dataVec.len)),
		Executable: bool(C.transaction_account_get_executable(accountPtr)),
		RentEpoch:  uint64(C.transaction_account_rent_epoch(accountPtr)),
	}
}

func GetTxContextAccounts(transactionCtx *C.c_transaction_context) (res []*types.Account) {
	accountArray := C.transaction_accounts_from_context(transactionCtx) //**c_transaction_account: accounts_ptr
	sz := unsafe.Sizeof(unsafe.Pointer(accountArray.accounts_ptr))
	res = make([]*types.Account, int(accountArray.len))
	for i := 0; i < int(accountArray.len); i++ {
		// pointer to the element = (accounts_ptr + i * sizeOf(pointer))
		elementPtr := (**C.c_transaction_account)(unsafe.Pointer(uintptr(unsafe.Pointer(accountArray.accounts_ptr)) + uintptr(i)*sz))
		res[i] = toGoAccount(*elementPtr)
	}

	C.transaction_accounts_free(accountArray)
	return res
}

func GetTxContextReturnData(transactionCtx *C.c_transaction_context) (pubKey []byte, data []byte) {
	returnData := C.transaction_return_data_from_context(transactionCtx)
	pubKey = C.GoBytes(unsafe.Pointer(returnData.pubkey), 32)
	data = C.GoBytes(unsafe.Pointer(returnData.return_data.data), C.int(returnData.return_data.len))
	return pubKey, data
}
