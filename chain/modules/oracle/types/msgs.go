package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgPushSimpleEntry{}

func (m *MsgPushSimpleEntry) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return err
	}
	
	if len(m.Entries) == 0 {
		return fmt.Errorf("empty entries array")
	}

	return nil
}

func (m MsgPushSimpleEntry) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
