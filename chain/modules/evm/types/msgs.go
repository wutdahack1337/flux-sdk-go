package types

import (
	"errors"
	fmt "fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgDeployContract{}
var _ sdk.Msg = &MsgExecuteContract{}

func (m *MsgDeployContract) ValidateBasic() error {
	if len(m.Bytecode) == 0 {
		return errors.New("byte code cannot be empty")
	}

	if len(m.InputAmount) > HashLen {
		return fmt.Errorf("input value must not be more than hash len of %d", HashLen)
	}

	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return fmt.Errorf("malformed sender: %w", err)
	}
	return nil
}

func (m *MsgDeployContract) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgExecuteContract) ValidateBasic() error {
	if len(m.ContractAddress) != EthAddressLen {
		return fmt.Errorf("contract address's len must be %d", EthAddressLen)
	}

	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return fmt.Errorf("malformed sender: %w", err)
	}
	return nil
}

func (m *MsgExecuteContract) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
