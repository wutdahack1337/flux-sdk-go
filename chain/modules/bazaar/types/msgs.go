package types

import (
	"cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/modules/fnft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var _ sdk.Msg = &MsgCreateProduct{}
var _ sdk.Msg = &MsgPurchaseOffering{}
var _ sdk.Msg = &MsgVerifyProduct{}

// ValidateBasic implements the Msg.ValidateBasic method.
func (m MsgCreateProduct) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid sender address (%s)", m.Sender)
	}
	if len(m.ClassId) == 0 {
		return fnfttypes.ErrEmptyClassID
	}
	if len(m.Id) == 0 {
		return fnfttypes.ErrEmptyNFTID
	}
	if len(m.Offerings) == 0 {
		return ErrEmptyOfferings
	}
	for _, o := range m.Offerings {
		if len(o.Url) > 0 {
			return ErrInvalidOfferingURL
		}
		if len(o.Price.Denom) == 0 {
			return ErrInvalidOfferingDenom
		}
		if o.Price.Amount.LTE(sdkmath.NewInt(0)) {
			return ErrInvalidOfferingAmount
		}
		if o.PurchaseCount != 0 {
			return ErrInvalidOfferingPurchaseCount
		}
	}
	return nil
}

// GetSigners returns the expected signers for MsgSend.
func (m MsgCreateProduct) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m MsgPurchaseOffering) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid sender address (%s)", m.Sender)
	}
	if len(m.ClassId) == 0 {
		return fnfttypes.ErrEmptyClassID
	}
	if len(m.Id) == 0 {
		return fnfttypes.ErrEmptyNFTID
	}
	if len(m.ProductId) == 0 {
		return ErrEmptyProductID
	}
	if len(m.OfferingIdx) == 0 {
		return ErrEmptyOfferings
	}
	if len(m.OfferingQuantity) == 0 {
		return ErrEmptyOfferings
	}
	if len(m.OfferingIdx) != len(m.OfferingQuantity) {
		return ErrMismatchOfferingLength
	}
	return nil
}

func (m MsgPurchaseOffering) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}

func (m MsgVerifyProduct) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return errors.Wrapf(sdkerrors.ErrInvalidAddress, "Invalid sender address (%s)", m.Sender)
	}
	if len(m.ClassId) == 0 {
		return fnfttypes.ErrEmptyClassID
	}
	if len(m.Id) == 0 {
		return fnfttypes.ErrEmptyNFTID
	}
	if len(m.ProductId) == 0 {
		return ErrEmptyProductID
	}
	return nil
}

func (m MsgVerifyProduct) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
