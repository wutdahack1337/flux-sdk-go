package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	types "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

// RegisterInterfaces registers the interfaces types with the interface registry.
func RegisterInterfaces(registry types.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgCreatePool{},
		&MsgUpdatePool{},
		&MsgDeposit{},
		&MsgWithdraw{},
	)

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec registers the necessary game interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgCreatePool{}, "interpool/MsgCreatePool", nil)
	cdc.RegisterConcrete(&MsgUpdatePool{}, "interpool/MsgUpdatePool", nil)
	cdc.RegisterConcrete(&MsgDeposit{}, "interpool/MsgDeposit", nil)
	cdc.RegisterConcrete(&MsgWithdraw{}, "interpool/MsgWithdraw", nil)
}
