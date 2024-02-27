package types

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgAstroTransfer{}

func (m MsgAstroTransfer) ValidateBasic() error {
	//if len(m.ClassId) == 0 {
	//	return fnfttypes.ErrEmptyClassID
	//}
	//if len(m.Id) == 0 {
	//	return fnfttypes.ErrEmptyNFTID
	//}
	//if len(m.GameId) == 0 {
	//	return ErrEmptyGameID
	//}
	//
	//if _, err := sdk.AccAddressFromBech32(m.Sender); err != nil {
	//	return err
	//}
	//if _, err := sdk.AccAddressFromBech32(m.From); err != nil {
	//	return err
	//}
	//if _, err := sdk.AccAddressFromBech32(m.To); err != nil {
	//	return err
	//}
	//
	//if len(m.Coin.Denom) == 0 || m.Coin.Amount.IsZero() {
	//	return ErrInvalidCoin
	//}

	return nil
}

func (m MsgAstroTransfer) GetSigners() []sdk.AccAddress {
	signer, _ := sdk.AccAddressFromBech32(m.Sender)
	return []sdk.AccAddress{signer}
}
