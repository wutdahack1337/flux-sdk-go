package types

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	sdkmath "cosmossdk.io/math"

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

		if err := m.Metadata.ValidateBasic(); err != nil {
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
			if err := m.Metadata.ValidateBasic(); err != nil {
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

func (m *StrategyMetadata) ValidateBasic() error {
	if m == nil {
		return fmt.Errorf("strategy metadata cannot be nil. Type must be defined")
	}

	switch m.Type {
	case StrategyType_STRATEGY:
	case StrategyType_INTENT_SOLVER:
		if err := validateSchema(m.Schema); err != nil {
			return err
		}
	case StrategyType_CRON:
		// validate cron gas price
		minimumGasPrice := sdkmath.NewIntFromUint64(500000000) // TODO: load from config
		if m.Type == StrategyType_CRON && m.CronGasPrice.LT(minimumGasPrice) {
			return fmt.Errorf("cron bot minimum gas price must greater than or equal chain minimum gas price: %s", minimumGasPrice.String())
		}

		var s interface{}
		if err := json.Unmarshal([]byte(m.CronInput), &s); err != nil {
			return fmt.Errorf("cron bot expects json string input")
		}

		// validate cron interval
		if m.CronInterval == 0 {
			return fmt.Errorf("cron bot timestamp interval cannot be zero")
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
