package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgCreatePool{}
var _ sdk.Msg = &MsgUpdatePool{}
var _ sdk.Msg = &MsgDeposit{}
var _ sdk.Msg = &MsgWithdraw{}

func (m *MsgCreatePool) ValidateBasic() error {
	return nil
}

func (m *MsgCreatePool) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgUpdatePool) ValidateBasic() error {
	return nil
}

func (m *MsgUpdatePool) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgDeposit) ValidateBasic() error {
	return nil
}

func (m *MsgDeposit) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgWithdraw) ValidateBasic() error {
	return nil
}

func (m *MsgWithdraw) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
