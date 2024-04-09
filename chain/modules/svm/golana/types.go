package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"
*/
import "C"
import (
	"encoding/hex"
	"fmt"
	"unsafe"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	"github.com/cosmos/btcutil/base58"
)

type ComputeBudget struct {
	computeBudget *C.c_compute_budget
}

func NewComputeBudget(computeUnitLimit uint64, maxInstructionTraceLength uint64, maxInvokeStackHeight uint64) ComputeBudget {
	return ComputeBudget{
		C.compute_budget_create(C.uint64_t(computeUnitLimit), C.size_t(maxInstructionTraceLength), C.size_t(maxInvokeStackHeight)),
	}
}

type Pubkeys struct {
	pubkeys *C.c_pubkeys
}

func NewPubkeys(pubkeys [][]byte) Pubkeys {
	cPubkeys := make([]*C.uchar, len(pubkeys))
	for i, pubkey := range pubkeys {
		cPubkeys[i] = (*C.uchar)(C.CBytes(pubkey))
	}
	return Pubkeys{
		C.pubkeys_create(
			(**C.uchar)(&cPubkeys[0]),
			C.ulong(len(pubkeys)),
		),
	}
}

type InstructionAccount struct {
	account *C.c_instruction_account
}

func NewInstructionAccount(
	transactionIndex uint16,
	callerIndex uint16,
	calleeIndex uint16,
	isSigner bool,
	isWritable bool) InstructionAccount {
	return InstructionAccount{
		C.instruction_account_create(
			C.ushort(transactionIndex),
			C.ushort(callerIndex),
			C.ushort(calleeIndex),
			C.bool(isSigner),
			C.bool(isWritable),
		),
	}
}

type IxInfo struct {
	ixInfo *C.c_ix_info
}

func NewIxInfo(ixAccounts []InstructionAccount, ixProgramAccounts []uint16, ixData []uint8) IxInfo {
	cIxAccounts := make([]*C.c_instruction_account, len(ixAccounts))
	for i, cia := range ixAccounts {
		cIxAccounts[i] = cia.account
	}
	var dataPtr *C.uchar
	if len(ixData) > 0 {
		dataPtr = (*C.uchar)(&ixData[0])
	}
	return IxInfo{
		C.ix_info_create(
			(**C.c_instruction_account)(&cIxAccounts[0]), C.ulong(len(cIxAccounts)),
			(*C.uint16_t)(&ixProgramAccounts[0]), C.ulong(len(ixProgramAccounts)),
			dataPtr, C.ulong(len(ixData)),
		),
	}
}

type TransactionAccount struct {
	account *C.c_transaction_account
}

func NewTransactionAccount(
	pubkey []byte,
	owner []byte,
	lamports uint64,
	data []byte,
	executable bool,
	rentEpoch uint64,
) TransactionAccount {
	pubkeyPtr := C.CBytes(pubkey)
	ownerPtr := C.CBytes(owner)
	dataPtr := C.CBytes(data)
	return TransactionAccount{
		C.transaction_account_create(
			(*C.uchar)(pubkeyPtr),
			(*C.uchar)(ownerPtr),
			C.uint64_t(lamports),
			(*C.uchar)(dataPtr), C.ulong(len(data)),
			C.bool(executable),
			C.uint64_t(rentEpoch),
		),
	}
}

func AccountDebug(a *types.Account) {
	fmt.Println("---\npubkey:", base58.Encode(a.Pubkey))
	fmt.Println("owner:", base58.Encode(a.Owner))
	fmt.Println("lamports:", a.Lamports)
	if len(a.Data) > 256 {
		fmt.Println("data len:", len(a.Data))
	} else {
		fmt.Println("data:", hex.EncodeToString(a.Data))
	}
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
