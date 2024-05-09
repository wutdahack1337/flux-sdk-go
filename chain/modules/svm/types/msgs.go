package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgTransaction{}

func (m *MsgTransaction) ValidateBasic() error {
	// parse signers and check their uniqueness
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return fmt.Errorf("signer is not valid bech32 format")
	}

	if m.ComputeBudget == 0 {
		return fmt.Errorf("compute budget cannot be zero")
	}

	if len(m.Accounts) == 0 {
		return fmt.Errorf("tx accounts array cannot be empty")
	}

	if len(m.Instructions) == 0 {
		return fmt.Errorf("tx instructions array cannot be empty")
	}

	// don't allow duplicate tx accounts
	txAccountsMap := map[string]struct{}{}
	for _, account := range m.Accounts {
		if _, exist := txAccountsMap[account]; exist {
			return fmt.Errorf("duplicate account in tx account list %s", account)
		} else {
			txAccountsMap[account] = struct{}{}
		}
	}

	// verify ix account indexes
	for _, ix := range m.Instructions {
		calleeIndexMap := map[uint32]uint32{}
		for idx, ixAccount := range ix.Accounts {
			if ixAccount.IdIndex > uint32(len(m.Accounts)) {
				return fmt.Errorf("ix account index out of range")
			}

			if ixAccount.CallerIndex > uint32(len(m.Accounts)) {
				return fmt.Errorf("ix account caller index out of range")
			}

			if ixAccount.CalleeIndex > uint32(len(ix.Accounts)) {
				return fmt.Errorf("ix account callee index of range")
			}

			if _, exist := calleeIndexMap[ixAccount.IdIndex]; !exist {
				calleeIndexMap[ixAccount.IdIndex] = uint32(idx)
			}

			if ixAccount.CalleeIndex != calleeIndexMap[ixAccount.IdIndex] {
				return fmt.Errorf("callee index must be the first position of the account in this instruction")
			}
		}
	}

	return nil
}

// This is for legacy version of cosmos sdk (< 0.5), for newer version, use the cosmos.v1.msg.signer option
func (m *MsgTransaction) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
