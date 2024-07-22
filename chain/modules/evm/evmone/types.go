package evmone

/*
#cgo CFLAGS: -I${SRCDIR}/../lib/include/ -I${SRCDIR}/../lib/include/
#cgo LDFLAGS: -Wl,-rpath,${SRCDIR}/../lib -L${SRCDIR}/../lib -levmone
#include "evmone/evmone.h"
#include "evmc/helpers.h"

*/
import "C"
import "context"

type Capability uint32

const (
	CapabilityEVM1  Capability = C.EVMC_CAPABILITY_EVM1
	CapabilityEWASM Capability = C.EVMC_CAPABILITY_EWASM
)

type CallKind int

const (
	Call         CallKind = C.EVMC_CALL
	DelegateCall CallKind = C.EVMC_DELEGATECALL
	CallCode     CallKind = C.EVMC_CALLCODE
	Create       CallKind = C.EVMC_CREATE
	Create2      CallKind = C.EVMC_CREATE2
)

type AccessStatus int

const (
	ColdAccess AccessStatus = C.EVMC_ACCESS_COLD
	WarmAccess AccessStatus = C.EVMC_ACCESS_WARM
)

type StorageStatus int

const (
	StorageAssigned         StorageStatus = C.EVMC_STORAGE_ASSIGNED
	StorageAdded            StorageStatus = C.EVMC_STORAGE_ADDED
	StorageDeleted          StorageStatus = C.EVMC_STORAGE_DELETED
	StorageModified         StorageStatus = C.EVMC_STORAGE_MODIFIED
	StorageDeletedAdded     StorageStatus = C.EVMC_STORAGE_DELETED_ADDED
	StorageModifiedDeleted  StorageStatus = C.EVMC_STORAGE_MODIFIED_DELETED
	StorageDeletedRestored  StorageStatus = C.EVMC_STORAGE_DELETED_RESTORED
	StorageAddedDeleted     StorageStatus = C.EVMC_STORAGE_ADDED_DELETED
	StorageModifiedRestored StorageStatus = C.EVMC_STORAGE_MODIFIED_RESTORED
)

type Hash [32]byte

func HashFromBytes(b []byte) Hash {
	var h Hash
	copy(h[:], b)
	return h
}

// Address represents the 160-bit (20 bytes) address of an Ethereum account.
type Address [20]byte

func AddressFromBytes(b []byte) Address {
	var a Address
	copy(a[:], b)
	return a
}

type Error int32

func (err Error) IsInternalError() bool {
	return err < 0
}

func (err Error) Error() string {
	return C.GoString(C.evmc_status_code_to_string(C.enum_evmc_status_code(err)))
}

const (
	Failure = Error(C.EVMC_FAILURE)
	Revert  = Error(C.EVMC_REVERT)
)

type Revision int32

const (
	Frontier             Revision = C.EVMC_FRONTIER
	Homestead            Revision = C.EVMC_HOMESTEAD
	TangerineWhistle     Revision = C.EVMC_TANGERINE_WHISTLE
	SpuriousDragon       Revision = C.EVMC_SPURIOUS_DRAGON
	Byzantium            Revision = C.EVMC_BYZANTIUM
	Constantinople       Revision = C.EVMC_CONSTANTINOPLE
	Petersburg           Revision = C.EVMC_PETERSBURG
	Istanbul             Revision = C.EVMC_ISTANBUL
	Berlin               Revision = C.EVMC_BERLIN
	London               Revision = C.EVMC_LONDON
	Paris                Revision = C.EVMC_PARIS
	Shanghai             Revision = C.EVMC_SHANGHAI
	Cancun               Revision = C.EVMC_CANCUN
	Prague               Revision = C.EVMC_PRAGUE
	MaxRevision          Revision = C.EVMC_MAX_REVISION
	LatestStableRevision Revision = C.EVMC_LATEST_STABLE_REVISION
)

type BytecodeExecutor interface {
	Execute(
		ctx HostContext, rev Revision,
		kind CallKind, static bool, depth int, gas int64,
		recipient Address, sender Address, input []byte, value Hash,
		code []byte,
	) (res Result, err error)
}

type HostKeeper interface {
	KVSetAccount(ctx context.Context, addr []byte, balance []byte)
	KVSetCode(ctx context.Context, addr, bytecode []byte)
	KVHasCode(ctx context.Context, addr []byte) bool
}

type VmKeeper interface {
	KVHasAccount(ctx context.Context, addr []byte) bool
	KVGetAccount(ctx context.Context, addr []byte) ([]byte, bool)
	KVGetStorage(ctx context.Context, addr, k []byte) ([]byte, bool)
	KVSetStorage(ctx context.Context, addr, k, v []byte)
	KVGetCode(ctx context.Context, addr []byte) ([]byte, bool)
	EmitExecutionEvent(ctx context.Context, address []byte)
	EmitLog(ctx context.Context, address []byte, topics [][]byte, data []byte)
}

type HostContext interface {
	AccountExists(addr Address) bool
	GetStorage(addr Address, key Hash) Hash
	SetStorage(addr Address, key Hash, value Hash) StorageStatus
	GetBalance(addr Address) Hash
	GetCodeSize(addr Address) int
	GetCodeHash(addr Address) Hash
	GetCode(addr Address) []byte
	SelfDestruct(addr Address, beneficiary Address) bool
	GetTxContext() TxContext
	GetBlockHash(number int64) Hash
	EmitLog(addr Address, topics [][]byte, data []byte)
	Call(kind CallKind,
		recipient Address, sender Address, value Hash, input []byte, gas int64, depth int,
		static bool, salt Hash, codeAddress Address) (output []byte, gasLeft int64, gasRefund int64,
		createAddr Address, err error)
	AccessAccount(addr Address) AccessStatus
	AccessStorage(addr Address, key Hash) AccessStatus
	GetTransientStorage(addr Address, key Hash) Hash
	SetTransientStorage(addr Address, key Hash, value Hash)
}

type Result struct {
	Output     []byte
	GasLeft    int64
	GasRefund  int64
	StatusCode string
}

// TxContext contains information about current transaction and block.
type TxContext struct {
	StoreKeeper VmKeeper
	CosmosCtx   context.Context

	GasPrice    Hash
	Origin      Address
	Coinbase    Address
	Number      int64
	Timestamp   int64
	GasLimit    int64
	PrevRandao  Hash
	ChainID     Hash
	BaseFee     Hash
	BlobBaseFee Hash

	bytecodeExecutor BytecodeExecutor
	hostKeeper       HostKeeper
	vmRevision       Revision

	transientStorage map[Address]map[Hash]Hash
}
