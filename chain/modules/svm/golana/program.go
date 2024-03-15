package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"
*/
import "C"

type RuntimeEnv struct {
	Env *C.c_program_runtime_env
}

func NewRuntimeEnv(computeBudget uint64) *RuntimeEnv {
	cComputeBudget := C.compute_budget_create(C.ulonglong(computeBudget))
	return &RuntimeEnv{
		C.program_runtime_create(cComputeBudget),
	}
}

type LoadedProgramsForTxBatch struct {
	Programs *C.c_loaded_programs_for_tx_batch
}

func NewLoadedProgramsForTxBatch(
	runtimeEnv *RuntimeEnv,
	accounts []*TransactionAccount,
) *LoadedProgramsForTxBatch {

	cAccounts := make([]*C.c_transaction_account, len(accounts))
	for i, acc := range accounts {
		cAccounts[i] = acc.Account
	}
	return &LoadedProgramsForTxBatch{
		C.loaded_programs_for_tx_batch_create(
			runtimeEnv.Env,
			(**C.c_transaction_account)(&cAccounts[0]),
			C.ulong(len(cAccounts)),
		),
	}
}

func NewModifiedProgramsByTxBatch(
	loaded *LoadedProgramsForTxBatch,
) *LoadedProgramsForTxBatch {
	return &LoadedProgramsForTxBatch{
		C.modified_programs_by_tx_batch_create(
			loaded.Programs,
		),
	}
}
