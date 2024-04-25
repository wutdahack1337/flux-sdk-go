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
		&MsgChargeVmAccount{},
		&MsgDrainVmAccount{},
		&MsgAstroTransfer{},
		&MsgFISTransaction{},
	)

	//registry.RegisterImplementations(
	//	(*govtypes.Content)(nil),
	//	&ClassCommissionsProposal{},
	//	&VerifiersProposal{},
	//)
	//
	//govtypes.RegisterProposalType((&ClassCommissionsProposal{}).ProposalType())
	//govtypes.RegisterProposalType((&VerifiersProposal{}).ProposalType())

	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

// RegisterLegacyAminoCodec registers the necessary game interfaces and concrete types
// on the provided LegacyAmino codec. These types are used for Amino JSON serialization.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	cdc.RegisterConcrete(&MsgChargeVmAccount{}, "astromesh/MsgChargeVmAccount", nil)
	cdc.RegisterConcrete(&MsgDrainVmAccountResponse{}, "astromesh/MsgDrainVmAccount", nil)
	cdc.RegisterConcrete(&MsgAstroTransfer{}, "astromesh/MsgAstroTransfer", nil)
	cdc.RegisterConcrete(&MsgFISTransaction{}, "astromesh/MsgFISTransaction", nil)
}
