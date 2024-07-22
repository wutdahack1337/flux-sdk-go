package types

import (
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgChargeVmAccount{}
var _ sdk.Msg = &MsgDrainVmAccount{}
var _ sdk.Msg = &MsgAstroTransfer{}
var _ sdk.Msg = &MsgFISTransaction{}

func (m *MsgChargeVmAccount) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("empty sender")
	}

	if _, exist := Plane_name[int32(m.Plane)]; !exist {
		return fmt.Errorf("unsupported vm plane: %d", m.Plane)
	}

	if err := m.Amount.Validate(); err != nil {
		return fmt.Errorf("coin format err: %w", err)
	}

	return nil
}

func (m MsgChargeVmAccount) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgDrainVmAccount) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("empty sender")
	}

	if _, exist := Plane_name[int32(m.Plane)]; !exist {
		return fmt.Errorf("unsupported vm plane: %d", m.Plane)
	}

	return nil
}

func (m MsgDrainVmAccount) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgAstroTransfer) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("empty sender")
	}

	if m.Receiver == "" {
		return fmt.Errorf("empty receiver")
	}

	if _, exist := Plane_name[int32(m.SrcPlane)]; !exist {
		return fmt.Errorf("unsupported source plane: %d", m.SrcPlane)
	}

	if _, exist := Plane_name[int32(m.DstPlane)]; !exist {
		return fmt.Errorf("unsupported dest plane: %d", m.DstPlane)
	}

	if err := m.Coin.Validate(); err != nil {
		return fmt.Errorf("coin format err: %w", err)
	}

	return nil
}

func (m MsgAstroTransfer) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m *MsgFISTransaction) ValidateBasic() error {
	if m.Sender == "" {
		return fmt.Errorf("empty sender")
	}

	for _, ix := range m.Instructions {
		if _, exist := Plane_name[int32(ix.Plane)]; !exist {
			return fmt.Errorf("unsupported vm plane: %d", ix.Plane)
		}
		if _, exist := TxAction_name[int32(ix.Action)]; !exist {
			return fmt.Errorf("unsupported instruction type: %d", ix.Action)
		}
	}
	return nil
}

func (m MsgFISTransaction) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
