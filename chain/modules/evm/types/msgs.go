package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgDeployContract{}
var _ sdk.Msg = &MsgExecuteBytecode{}

func (m MsgDeployContract) ValidateBasic() error {
	// TODO
	return nil
}

func (m MsgDeployContract) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m MsgExecuteBytecode) ValidateBasic() error {
	// TODO
	return nil
}

func (m MsgExecuteBytecode) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
