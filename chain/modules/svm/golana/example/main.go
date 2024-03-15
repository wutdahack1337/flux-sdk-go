package main

/*
#cgo LDFLAGS: -L./../../lib -lgolana
#include "./../../lib/golana.h"
*/
import "C"

import (
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/golana"
	"os"

	"github.com/cosmos/btcutil/base58"
	solanago "github.com/gagliardetto/solana-go"
)

func main() {
	systemProgramId := base58.Decode("11111111111111111111111111111111")
	bpfProgramId := base58.Decode("BPFLoader2111111111111111111111111111111111")
	helloProgramId := []byte{8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8}
	helloProgramDataId := []byte{1, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8}
	senderId, _ := solanago.NewRandomPrivateKey()

	programBz, err := os.ReadFile("example.so")
	if err != nil {
		panic(err)
	}

	accounts := []*golana.TransactionAccount{
		golana.NewTransactionAccount(helloProgramId, bpfProgramId, 100000, programBz, true, 100),
		golana.NewTransactionAccount(helloProgramDataId, helloProgramId, 200000, []byte{0, 0, 0, 0}, false, 100),
		golana.NewTransactionAccount(senderId, systemProgramId, 200000, []byte{1, 2, 3, 4}, false, 100),
	}

	computeBudget := uint64(100000)
	runtimeEnv := golana.NewRuntimeEnv(computeBudget)
	loadedPrograms := golana.NewLoadedProgramsForTxBatch(runtimeEnv, accounts)
	modifiedPrograms := golana.NewModifiedProgramsByTxBatch(loadedPrograms)

	txCtx := golana.NewTransactionContext(computeBudget, accounts)
	invokeCtx := golana.NewInvokeContext(computeBudget, txCtx, loadedPrograms, modifiedPrograms)

	programIds := []uint16{0}
	ixAccounts := []*golana.InstructionAccount{
		golana.NewInstructionAccount(1, 0, 0, true, true),
		golana.NewInstructionAccount(2, 1, 1, true, true),
	}
	ixData := []byte{0}

	computeUnitConsumed, err := invokeCtx.ProcessInstruction(programIds, ixAccounts, ixData)
	fmt.Println(computeUnitConsumed, err)

	goAccounts := golana.GetTxContextAccounts(txCtx.TxContext)
	for _, a := range goAccounts {
		a.Debug()
	}
}
