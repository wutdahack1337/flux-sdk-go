package main

import (
	"bytes"
	"encoding/binary"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type WriteBuffer struct {
	Offset uint32
	Data   []byte
}

func MarshalIxData(s interface{}) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := bin.NewBinEncoder(buf)
	err := enc.Encode(s)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func MustMarshalIxData(s interface{}) []byte {
	bz, err := MarshalIxData(s)
	if err != nil {
		panic(err)
	}

	return bz
}

func (inst WriteBuffer) MarshalWithEncoder(encoder *bin.Encoder) error {
	if err := encoder.WriteInt32(int32(1), binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteUint32(inst.Offset, binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteUint64(uint64(len(inst.Data)), binary.LittleEndian); err != nil {
		return err
	}

	if err := encoder.WriteBytes(inst.Data, false); err != nil {
		return err
	}

	return nil
}

type DeployWithMaxDataLen struct {
	DataLen uint64
}

func (inst DeployWithMaxDataLen) MarshalWithEncoder(encoder *bin.Encoder) error {
	if err := encoder.WriteInt32(int32(2), binary.LittleEndian); err != nil {
		return err
	}

	return encoder.WriteUint64(inst.DataLen, binary.LittleEndian)
}

func ToCosmosMsg(signer string, computeBudget uint64, tx *solana.Transaction) *types.MsgTransaction {
	pubkeys := []string{}
	for _, p := range tx.Message.AccountKeys {
		pubkeys = append(pubkeys, p.String())
	}

	ixs := []*types.Instruction{}
	for _, ix := range tx.Message.Instructions {
		fluxInstr := &types.Instruction{
			ProgramIndex: []uint32{uint32(ix.ProgramIDIndex)},
			Data:         ix.Data,
		}

		accounts, err := ix.ResolveInstructionAccounts(&tx.Message)
		if err != nil {
			panic(err)
		}

		for _, a := range accounts {
			fluxInstr.Accounts = append(fluxInstr.Accounts, &types.InstructionAccount{
				IdIndex:     uint32(positionOf(a.PublicKey, tx.Message.AccountKeys)),
				CallerIndex: uint32(positionOf(a.PublicKey, tx.Message.AccountKeys)),
				CalleeIndex: uint32(positionOfPubkeyInAccountMetas(a.PublicKey, accounts)),
				IsSigner:    a.IsSigner,
				IsWritable:  a.IsWritable,
			})
		}
		ixs = append(ixs, fluxInstr)
	}

	return &types.MsgTransaction{
		Sender:        signer,
		Accounts:      pubkeys,
		Instructions:  ixs,
		ComputeBudget: computeBudget,
	}
}

func positionOf(a solana.PublicKey, s []solana.PublicKey) int {
	for i, pk := range s {
		if pk.Equals(a) {
			return i
		}
	}
	return -1
}

func positionOfPubkeyInAccountMetas(a solana.PublicKey, metas []*solana.AccountMeta) int {
	for idx, meta := range metas {
		if meta.PublicKey.Equals(a) {
			return idx
		}
	}

	return -1
}
