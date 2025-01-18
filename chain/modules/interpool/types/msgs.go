package types

import (
	"encoding/hex"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgCreatePool{}
var _ sdk.Msg = &MsgUpdatePool{}
var _ sdk.Msg = &MsgDeposit{}
var _ sdk.Msg = &MsgWithdraw{}

func (m *MsgCreatePool) ValidateBasic() error {
	if m.OperatorCommissionConfig == nil {
		return fmt.Errorf("require commission config")
	}
	return nil
}

func (m *MsgCreatePool) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgUpdatePool) ValidateBasic() error {
	_, err := hex.DecodeString(m.PoolId)
	if err != nil {
		return fmt.Errorf("pool id is not valid hex: %w", err)
	}

	_, err = hex.DecodeString(m.CronId)
	if err != nil {
		return fmt.Errorf("cron id is not valid hex: %w", err)
	}
	return nil
}

func (m *MsgUpdatePool) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgDeposit) ValidateBasic() error {
	_, err := hex.DecodeString(m.PoolId)
	if err != nil {
		return fmt.Errorf("pool id is not valid hex: %w", err)
	}

	err = sdk.Coins(m.DepositSnapshot).Validate()
	if err != nil {
		return fmt.Errorf("deposit snapshot validate err: %w", err)
	}
	return nil
}

func (m *MsgDeposit) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgWithdraw) ValidateBasic() error {
	_, err := hex.DecodeString(m.PoolId)
	if err != nil {
		return fmt.Errorf("pool id is not valid hex: %w", err)
	}

	if !m.Percentage.IsPositive() || m.Percentage.GT(math.NewInt(PercentagePrecision)) {
		return fmt.Errorf("percentage must be in range 1..%d (0.01% .. 100%%)", PercentagePrecision)
	}
	return nil
}

func (m *MsgWithdraw) GetSigners() (signers []sdk.AccAddress) {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
