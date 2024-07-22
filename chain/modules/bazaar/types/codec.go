package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	types "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// RegisterInterfaces registers the interfaces types with the interface registry.
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreateProduct{},
		&MsgPurchaseOffering{},
	)

	registry.RegisterImplementations(
		(*govtypes.Content)(nil),
		&ClassCommissionsProposal{},
		&VerifiersProposal{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec registers the necessary bazaar interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreateProduct{}, "bazaar/MsgCreateProduct", nil)
	cdc.RegisterConcrete(&MsgVerifyProduct{}, "bazaar/MsgVerifyProduct", nil)
	cdc.RegisterConcrete(&MsgPurchaseOffering{}, "bazaar/MsgPurchaseOffering", nil)
}
