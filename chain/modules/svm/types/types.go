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

	// Maximum cross-program invocation and instructions per transaction
	DefaultMaxInstructionTraceLength uint64 = 2048

	// From solana: Maximum program instruction invocation stack height
	// It's same as EVM call depth
	DefaultMaxInvokeStackHeight   uint64  = 8
	DefaultStackFrameSize                 = 8192
	DefaultHeapSize                       = 32 * 1024
	DefaultLamportsPerByteYear    uint64  = 1000000000 * 365 / 100 / (1024 * 1024)
	DefaultExemptionThreshold     float64 = 2.0
	DefaultBurnPercent            byte    = 50
	DefaultAccountStorageOverhead uint64  = 128

	DefaultTickPerDay               uint64 = 160 * (24 * 60 * 60) // tick per second * second per day
	DefaultTickPerSlot              uint64 = 64
	DefaultSlotsPerEpoch            uint64 = 2 * DefaultTickPerDay / DefaultTickPerSlot
	DefaultLeaderScheduleSlotOffset uint64 = DefaultSlotsPerEpoch
)

// rent exempt amount  = lamports per byte per year * (DefaultAccountStorageOverhead+size in byte) * 2 years
// see solana Rent::minimum_balance
func GetRentExemptLamportAmount(size uint64) uint64 {
	return 3480 * (DefaultAccountStorageOverhead + size) * 2
}

var (
	TransientStoreKey = fmt.Sprintf("transient_%s", StoreKey)

	NativeProgramId           = solana.MustPublicKeyFromBase58("NativeLoader1111111111111111111111111111111")
	SplToken2022ProgramId     = solana.MustPublicKeyFromBase58("TokenzQdBNbLqP5VEhdkAS6EPFLC1PHnBqCXEpPxuEb")
	AssociatedTokenProgramId  = solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	UpgradableLoaderProgramId = solana.MustPublicKeyFromBase58("BPFLoaderUpgradeab1e11111111111111111111111")
	SystemProgramId           = solana.MustPublicKeyFromBase58("11111111111111111111111111111111")
	SplTokenProgramId         = solana.MustPublicKeyFromBase58("TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA")

	// sysvars
	SysVarRent          = solana.MustHashFromBase58("SysvarRent111111111111111111111111111111111")
	SysVarEpochSchedule = solana.MustHashFromBase58("SysvarEpochSchedu1e111111111111111111111111")
)
