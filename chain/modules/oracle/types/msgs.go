package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgPushSimpleEntry{}

func (m *MsgPushSimpleEntry) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("empty sender")
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
