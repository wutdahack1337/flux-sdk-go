package golana

/*
#cgo LDFLAGS: -L./../lib -lgolana
#include "./../lib/golana.h"
*/
import "C"
import (
	"fmt"
	goerrors "github.com/pkg/errors"
)

type TransactionContext struct {
	TxContext *C.c_transaction_context
}

func NewTransactionContext(
	computeBudget uint64,
	accounts []*TransactionAccount,
) *TransactionContext {
	cComputeBudget := C.compute_budget_create(C.ulonglong(computeBudget))
	cAccounts := make([]*C.c_transaction_account, len(accounts))
	for i, acc := range accounts {
		cAccounts[i] = acc.Account
	}

	return &TransactionContext{
		C.transaction_context_create(
			cComputeBudget,
			(**C.c_transaction_account)(&cAccounts[0]),
			C.ulong(len(cAccounts)),
		),
	}
}

func (c *TransactionContext) Free() error {
	ptr := c.TxContext
	if ptr == nil {
		return goerrors.New("nil pointer on c_transaction account")
	}

	C.transaction_context_free(ptr)
	return nil
}

type InvokeContext struct {
	invokeCtx *C.c_invoke_context
}

func NewInvokeContext(
	computeBudget uint64,
	txCtx *TransactionContext,
	loadedPrograms *LoadedProgramsForTxBatch,
	modifiedPrograms *LoadedProgramsForTxBatch,
) *InvokeContext {
	cComputeBudget := C.compute_budget_create(C.ulonglong(computeBudget))
	cSysvarCache := C.sysvar_cache_create()
	return &InvokeContext{
		C.invoke_context_create(
			txCtx.TxContext,
			cSysvarCache,
			cComputeBudget,
			loadedPrograms.Programs,
			modifiedPrograms.Programs,
		),
	}
}

func (c *InvokeContext) ProcessInstruction(
	programIds []uint16,
	ixAccounts []*InstructionAccount,
	ixData []byte,
) (uint64, error) {

	computeUnitConsumed := C.ulonglong(0)
	cErr := C.invoke_context_process_instruction(
		c.invokeCtx,
		(*C.uchar)(&ixData[0]), C.ulong(len(ixData)),
		(*C.uint16_t)(&programIds[0]), C.ulong(len(programIds)),
		(**C.c_instruction_account)(&ixAccounts[0].Account), C.ulong(len(ixAccounts)),
		&computeUnitConsumed,
	)

	if cErr != C.Ok {
		return 0, fmt.Errorf("failed to process instruction")
	}

	return uint64(computeUnitConsumed), nil
}

func (c *InvokeContext) Free() error {
	ptr := c.invokeCtx
	if ptr == nil {
		return goerrors.New("nil pointer err")
	}

	C.invoke_context_free(ptr)
	return nil
}
