package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgConfigStrategy{}
var _ sdk.Msg = &MsgTriggerStrategies{}

func (m *MsgConfigStrategy) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return err
	}

	switch m.Config {
	case Config_deploy, Config_update:
		if len(m.Strategy) == 0 {
			return fmt.Errorf("invalid strategy binary for deploy, update options")
		}
		if len(m.Id) > 0 {
			return fmt.Errorf("strategy id should be empty for deploy, update options")
		}
	case Config_disable, Config_revoke:
		if len(m.Strategy) > 0 {
			return fmt.Errorf("strategy should be empty for disable, revoke options")
		}
		if len(m.Id) == 0 {
			return fmt.Errorf("invalid strategy id for disable, revoke options")
		}
	default:
		return fmt.Errorf("invalid strategy config option")
	}
	return nil
}

func (m MsgConfigStrategy) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgTriggerStrategies) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return err
	}
	if len(m.Ids) == 0 {
		return fmt.Errorf("empty strategy ids")
	}
	if len(m.Inputs) == 0 {
		return fmt.Errorf("empty strategy inputs")
	}
	return nil
}

func (m MsgTriggerStrategies) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
