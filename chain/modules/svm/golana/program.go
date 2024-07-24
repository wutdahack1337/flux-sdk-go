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
	"github.com/gagliardetto/solana-go"
)

func GetBuiltinProgramIDs() (res [][]byte) {
	programIdByteArray := C.golana_get_builtins_program_keys()
	goByteArray := C.GoBytes(unsafe.Pointer(programIdByteArray.data), C.int(programIdByteArray.len))
	builtinCount := int(programIdByteArray.len) / 32
	for i := 0; i < builtinCount; i++ {
		bytes := goByteArray[i*32 : (i+1)*32]
		res = append(res, bytes)
	}
	C.golana_bytes_free(programIdByteArray)
	return res
}

type ProgramAccountMeta struct {
	TypeId                  uint32
	ProgramExecutablePubkey solana.PublicKey
}

type ProgramExecutableMeta struct {
	TypeId       uint32
	Slot         uint64
	HasAuthority bool
	Authority    *solana.PublicKey
}

// this returns 2 accounts owned by `owner`
func ComposeProgramDataAccounts(programPubkey solana.PublicKey, loaderOwner solana.PublicKey, programBz []byte) (res []*types.Account, err error) {
	// compose program account
	programExecutableDataPubkey, _, err := solana.FindProgramAddress(
		[][]byte{
			programPubkey[:],
		},
		loaderOwner,
	)
	if err != nil {
		return nil, fmt.Errorf("cannot find PDA of %s: %w", programPubkey.String(), err)
	}

	programDataBz, err := MarshalBinary(ProgramAccountMeta{
		TypeId:                  2,
		ProgramExecutablePubkey: programExecutableDataPubkey,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal program account meta: %w", err)
	}
	res = append(res, &types.Account{
		Pubkey:     programPubkey[:],
		Owner:      loaderOwner[:],
		Lamports:   types.GetRentExemptLamportAmount(uint64(len(programDataBz))),
		Data:       programDataBz,
		Executable: true,
		RentEpoch:  0,
	})

	programExecutableMetaBz, err := MarshalBinary(ProgramExecutableMeta{
		TypeId:       3,
		Slot:         0,
		HasAuthority: true,
		Authority:    &solana.SystemProgramID,
	})
	if err != nil {
		return nil, fmt.Errorf("cannot marshal program executable data account meta: %w", err)
	}

	// compose program data account
	res = append(res, &types.Account{
		Pubkey:     programExecutableDataPubkey[:],
		Owner:      loaderOwner[:],
		Lamports:   types.GetRentExemptLamportAmount(uint64(len(programExecutableMetaBz) + len(programBz))),
		Data:       append(programExecutableMetaBz, programBz...),
		Executable: false, // only program account has executable flag = true
		RentEpoch:  0,
	})
	return res, nil
}
