package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ sdk.Msg = &MsgConfigStrategy{}
var _ sdk.Msg = &MsgTriggerStrategies{}

var templateVarReg, _ = regexp.Compile(`(\${[^}]+})`)

func (m *MsgConfigStrategy) ValidateBasic() error {
	_, err := sdk.AccAddressFromBech32(m.Sender)
	if err != nil {
		return err
	}

	switch m.Config {
	case Config_deploy:
		if len(m.Strategy) == 0 {
			return fmt.Errorf("strategy binary for deploy must not be empty")
		}

		if len(m.Id) > 0 {
			return fmt.Errorf("strategy id should be empty for deploy, update options")
		}

		if err := m.Metadata.ValidateBasic(m.Query, true); err != nil {
			return fmt.Errorf("metadata validate error: %w", err)
		}

	case Config_update:
		if len(m.Strategy) > 0 {
			return fmt.Errorf("strategy binary must be empty")
		}

		if len(m.Id) == 0 {
			return fmt.Errorf("invalid strategy id to update")
		}

		_, err := hex.DecodeString(m.Id)
		if err != nil {
			return fmt.Errorf("strategy id hex parse err: %w", err)
		}

		if m.Metadata != nil {
			if err := m.Metadata.ValidateBasic(m.Query, false); err != nil {
				return fmt.Errorf("metadata validate error: %w", err)
			}
		}
	case Config_disable, Config_revoke, Config_enable:
		if len(m.Strategy) > 0 {
			return fmt.Errorf("strategy should be empty for enable, disable, revoke options")
		}

		if len(m.Id) == 0 {
			return fmt.Errorf("invalid strategy id for enable, disable, revoke options")
		}

		_, err := hex.DecodeString(m.Id)
		if err != nil {
			return fmt.Errorf("strategy id hex parse err: %w", err)
		}

		if m.Query != nil {
			return fmt.Errorf("query not allowed in enable, disable and revoke")
		}

		if m.Metadata != nil {
			return fmt.Errorf("metadata config not allowed in enable, enable and revoke")
		}
	default:
		return fmt.Errorf("invalid strategy config option: %s", m.Config.String())
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

func (m *StrategyMetadata) ValidateBasic(query *types.FISQueryRequest, requireAppsUnverified bool) error {
	if m == nil {
		return fmt.Errorf("strategy metadata cannot be nil. Type must be defined")
	}

	// ensure supported apps are not duplicated
	seenApps := map[string]struct{}{}
	for _, v := range m.SupportedApps {
		contractAddress, err := ParseContractAddr(v.ContractAddress, v.Plane)
		if err != nil {
			return fmt.Errorf("invalid contract address: %s (plane: %s), err: %w", v.ContractAddress, v.Plane.String(), err)
		}

		appId := v.Plane.String() + string(contractAddress)
		if _, seen := seenApps[appId]; seen {
			return fmt.Errorf("app contract duplicated: %s, plane: %s", v.ContractAddress, v.Plane)
		}
		seenApps[appId] = struct{}{}
	}

	// ensure all apps are not verified in config phase
	if requireAppsUnverified {
		for _, v := range m.SupportedApps {
			if v.Verified {
				return fmt.Errorf("app must not be verified: %s, plane: %s", v.Name, v.Plane)
			}
		}
	}

	switch m.Type {
	case StrategyType_STRATEGY:
	case StrategyType_INTENT_SOLVER:
		if err := validateSchema(m.Schema); err != nil {
			return err
		}
	case StrategyType_CRON:
		// validate cron gas price
		minimumGasPrice := CRON_MINIMUM_GAS_PRICE
		if m.Type == StrategyType_CRON && m.CronGasPrice.LT(minimumGasPrice) {
			return fmt.Errorf("cron bot minimum gas price must greater than or equal chain minimum gas price: %s", minimumGasPrice.String())
		}

		var s interface{}
		if err := json.Unmarshal([]byte(m.CronInput), &s); err != nil {
			return fmt.Errorf("cron bot expects json string input")
		}

		// validate cron interval
		if m.CronInterval == 0 {
			if len(query.Instructions) == 0 {
				return fmt.Errorf("event-based cron service should have at least 1 fis event query")
			}
			validQuery := false
			for _, q := range query.Instructions {
				if q.Action == types.QueryAction_COSMOS_EVENT {
					validQuery = true
					break
				}
			}
			if !validQuery {
				return fmt.Errorf("event-based cron service doesn't have any fis event query")
			}
		}
	default:
		return fmt.Errorf("invalid value for mandatory field strategy.Metadata.Type: %s", m.Type.String())
	}

	return nil
}

func validateSchema(schemaString string) error {
	if schemaString == "" {
		return nil
	}

	var s Schema
	if err := json.Unmarshal([]byte(schemaString), &s); err != nil {
		return err
	}

	for _, g := range s.Groups {
		if len(s.Groups) > 1 && g.Name == "" {
			return fmt.Errorf("group name should not be empty when there are many groups")
		}
		for _, config := range g.Prompts {
			if err := config.ValidateBasic(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *SchemaPrompt) ValidateBasic() error {
	template := p.Template // template is input from user
	fillableVars := templateVarReg.FindAllString(template, -1)
	for _, v := range fillableVars {
		// ${var_name:some_type}
		// we won't check type for now, FE will be able to parse it, ignore unknown type
		definition := v[2 : len(v)-1]
		colonIndex := strings.IndexRune(definition, ':')
		if colonIndex <= 0 || colonIndex == len(definition)-1 {
			// ${name}, ${:name} and ${name:} are invalid
			return fmt.Errorf("invalid definition, must conform structure ${name:type}: %s", v)
		}

		if strings.IndexRune(definition, ' ') >= 0 {
			return fmt.Errorf("invalid definition, must contains no spaces: %s", v)
		}
	}

	return nil
}
