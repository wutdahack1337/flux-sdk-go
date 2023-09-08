package types

import (
	fnfttypes "github.com/FluxNFTLabs/sdk-go/chain/modules/fnft/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	"github.com/cosmos/gogoproto/proto"
	goerrors "github.com/pkg/errors"
)

var _ govtypes.Content = &UpdateClassCommissionsProposal{}

func init() {
	govtypes.RegisterProposalType(proto.MessageName(&UpdateClassCommissionsProposal{}))
}

func NewUpdateClassCommissionsProposal(title, description string, req []ClassCommission) *UpdateClassCommissionsProposal {
	return &UpdateClassCommissionsProposal{
		Title:            title,
		Description:      description,
		ClassCommissions: req,
	}
}

func (p *UpdateClassCommissionsProposal) GetTitle() string {
	return p.Title
}

func (p *UpdateClassCommissionsProposal) GetDescription() string {
	return p.Description
}

func (p *UpdateClassCommissionsProposal) ProposalRoute() string {
	return RouterKey
}

func (p *UpdateClassCommissionsProposal) ProposalType() string {
	return proto.MessageName(p)
}

// ValidateBasic returns ValidateBasic result of this proposal.
func (p *UpdateClassCommissionsProposal) ValidateBasic() error {
	if len(p.ClassCommissions) == 0 {
		return goerrors.New("commissions array cannot be empty")
	}
	for _, cc := range p.ClassCommissions {
		if len(cc.ClassId) == 0 {
			return fnfttypes.ErrEmptyClassID
		}
		if cc.CommissionDiv == 0 || cc.CommissionMul == 0 {
			return goerrors.New("commission parts cannot be 0")
		}
	}
	return govtypes.ValidateAbstract(p)
}
