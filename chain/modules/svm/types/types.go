package types

import (
	fmt "fmt"

	"github.com/gagliardetto/solana-go"
)

const (
	// ModuleName defines the name of the svm module
	ModuleName = "svm"

	// StoreKey is the default store key for svm
	StoreKey = ModuleName

	// RouterKey is the message route for svm
	RouterKey = ModuleName

	HashLen       = 32
	EthAddressLen = 20

	// in solana DEFAULT_LAMPORTS_PER_BYTE_YEAR = ~3480 lamports, number of year for being rent exempt is 2
	// => 10MB needs 3480 * 10 * 1024 * 1024 * 2, multiplied to three to have a redundant value
	DefaultLamportForRentExempt uint64 = 3480 * 10 * 1024 * 1024 * 3

	// Maximum cross-program invocation and instructions per transaction
	DefaultMaxInstructionTraceLength uint64 = 512

	// From solana: Maximum program instruction invocation stack height
	// It's same as EVM call depth
	DefaultMaxInvokeStackHeight uint64 = 5

	DefaultLamportsPerByteYear uint64  = 1000000000 / 100 * 365 / (1024 * 1024)
	DefaultExemptionThreshold  float64 = 2.0
	DefaultBurnPercent         byte    = 50

	DefaultAccountStorageOverhead uint64 = 128
)

var (
	TransientStoreKey = fmt.Sprintf("transient_%s", StoreKey)

	NativeProgramId           = solana.MustPublicKeyFromBase58("NativeLoader1111111111111111111111111111111")
	SplToken2022ProgramId     = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	AssociatedTokenProgramId  = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	UpgradableLoaderProgramId = solana.MustPublicKeyFromBase58("BPFLoaderUpgradeab1e11111111111111111111111")
	SystemProgramId           = solana.MustPublicKeyFromBase58("11111111111111111111111111111111")

	// sysvars
	SysVarRent = solana.MustHashFromBase58("SysvarRent111111111111111111111111111111111")
)
