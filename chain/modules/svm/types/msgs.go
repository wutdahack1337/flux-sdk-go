package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgCreateAccount{}
var _ sdk.Msg = &MsgTransaction{}

func (m *MsgCreateAccount) ValidateBasic() error {
	// TODO
	return nil
}

func (m *MsgCreateAccount) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgTransaction) ValidateBasic() error {
	// TODO
	return nil
}

func (m *MsgTransaction) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
