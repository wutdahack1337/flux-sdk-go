package types

import (
	sdkmath "cosmossdk.io/math"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

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

	case Config_disable, Config_revoke, Config_enable, Config_update:
		if len(m.Strategy) > 0 {
			return fmt.Errorf("strategy should be empty for disable, revoke options")
		}
		if len(m.Id) == 0 {
			return fmt.Errorf("invalid strategy id for disable, revoke options")
		}

		_, err := hex.DecodeString(m.Id)
		if err != nil {
			return fmt.Errorf("strategy id hex parse err: %w", err)
		}

		if err := m.Metadata.ValidateBasic(); err != nil {
			return fmt.Errorf("metadata validate error: %w", err)
		}
	default:
		return fmt.Errorf("invalid strategy config option")
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
		return nil
	}

	if err := validateSchema(m.Schema); err != nil {
		return err
	}

	// TODO: load from config
	minimumGasPrice := sdkmath.NewIntFromUint64(500000000)
	if m.Type == StrategyType_CRON && m.CronGasPrice.LT(minimumGasPrice) {
		return fmt.Errorf("cron bot minimum gas price must greater than or equal chain minimum gas price: %s", minimumGasPrice.String())
	}

	// validate cron input
	var s interface{}
	if err := json.Unmarshal([]byte(m.CronInput), &s); err != nil {
		return fmt.Errorf("cron bot expects json string input")
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
