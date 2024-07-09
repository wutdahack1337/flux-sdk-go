package main

import (
	"context"
	"fmt"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"os"
	"strings"

	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678\n"),
		chainclient.GetCryptoCodec(),
	)
	if err != nil {
		panic(err)
	}

	// init grpc connection
	cc, err := grpc.Dial(network.ChainGrpcEndpoint, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}

	// init client ctx
	clientCtx, senderAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"user1",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx = clientCtx.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	// init wasm client
	wasmClient := wasmtypes.NewQueryClient(cc)

	// read codes
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	astroportFactoryCode, err := os.ReadFile(dir + "/examples/chain/24_DeployAstroport/artifacts/astroport_factory.wasm")
	if err != nil {
		panic(err)
	}
	astroportXYKPairCode, err := os.ReadFile(dir + "/examples/chain/24_DeployAstroport/artifacts/astroport_pair.wasm")
	if err != nil {
		panic(err)
	}
	astroportRouterCode, err := os.ReadFile(dir + "/examples/chain/24_DeployAstroport/artifacts/astroport_router.wasm")
	if err != nil {
		panic(err)
	}
	cw20BaseCode, err := os.ReadFile(dir + "/examples/chain/24_DeployAstroport/artifacts/cw20_base.wasm")
	if err != nil {
		panic(err)
	}
	astroportCodes := [][]byte{
		astroportFactoryCode,
		astroportXYKPairCode,
		astroportRouterCode,
		cw20BaseCode,
	}

	astroportCodeIds := []uint64{}

	// store astroport factory code
	for _, code := range astroportCodes {
		res, err := chainClient.SyncBroadcastMsg(&wasmtypes.MsgStoreCode{
			Sender:       senderAddress.String(),
			WASMByteCode: code,
			InstantiatePermission: &wasmtypes.AccessConfig{
				Permission: wasmtypes.AccessTypeEverybody,
				Addresses:  nil,
			},
		})
		if err != nil {
			panic(err)
		}
		var msgData sdk.TxMsgData
		var storeCodeRes wasmtypes.MsgStoreCodeResponse
		resBz := ethcommon.Hex2Bytes(res.TxResponse.Data)
		proto.Unmarshal(resBz, &msgData)
		proto.Unmarshal(msgData.MsgResponses[0].Value, &storeCodeRes)
		astroportCodeIds = append(astroportCodeIds, storeCodeRes.CodeID)
	}

	// assign code id to variables
	astroportFactoryCodeId := astroportCodeIds[0]
	astroportXYKPairCodeId := astroportCodeIds[1]
	astroportRouterCodeId := astroportCodeIds[2]
	cw20BaseCodeId := astroportCodeIds[3]
	fmt.Println("astroportFactoryCodeId:", astroportFactoryCodeId)
	fmt.Println("astroportXYKPairCodeId:", astroportXYKPairCodeId)
	fmt.Println("astroportRouterCodeId:", astroportRouterCodeId)
	fmt.Println("cw20BaseCodeId:", cw20BaseCodeId)

	// instantiate astroport factory contract
	res, err := chainClient.SyncBroadcastMsg(&wasmtypes.MsgInstantiateContract{
		Sender: senderAddress.String(),
		Admin:  senderAddress.String(),
		CodeID: astroportFactoryCodeId,
		Label:  "Astroport Factory Contract",
		Msg: []byte(fmt.Sprintf(`{
			"token_code_id": %d,
			"fee_address": "lux1rgxjfea3y2e7n0frz5syly8n5zulagy3v2kd2j",
			"owner": "lux154evaeem8veltk78y6athyzya6wwzhvu33wzpl",
			"generator_address": "lux158ucxjzr6ccrlpmz8z05wylu8tr5eueqcp2afu",
			"whitelist_code_id": 0,
			"coin_registry_address": "lux158ucxjzr6ccrlpmz8z05wylu8tr5eueqcp2afu",
			"pair_configs": [
			  {
				"code_id": %d,
				"pair_type": {
				  "xyk": {}
				},
				"total_fee_bps": 100,
				"maker_fee_bps": 10,
				"is_disabled": false
			  }
			]
		}`, cw20BaseCodeId, astroportXYKPairCodeId)),
		Funds: nil,
	})
	if err != nil {
		panic(err)
	}
	var msgData sdk.TxMsgData
	var instantiateRes wasmtypes.MsgInstantiateContractResponse
	resBz := ethcommon.Hex2Bytes(res.TxResponse.Data)
	proto.Unmarshal(resBz, &msgData)
	proto.Unmarshal(msgData.MsgResponses[0].Value, &instantiateRes)
	astroportFactoryContract := instantiateRes.Address
	fmt.Println("astroport factory contract:", astroportFactoryContract)

	// create xyk pairs from factory contracts
	baseDenoms := []string{"btc", "eth", "sol"}
	quoteDenom := "usdt"
	for _, baseDenom := range baseDenoms {
		res, err = chainClient.SyncBroadcastMsg(&wasmtypes.MsgExecuteContract{
			Sender:   senderAddress.String(),
			Contract: astroportFactoryContract,
			Msg: []byte(fmt.Sprintf(`{
		  "create_pair": {
			"pair_type": {
			  "xyk": {}
			},
			"asset_infos": [
			  {
				"native_token": {
				  "denom": "%s"
				}
			  },
			  {
				"native_token": {
				  "denom": "%s"
				}
			  }
			]
		  }
		}`, baseDenom, quoteDenom)),
			Funds: nil,
		})
		if err != nil {
			panic(err)
		}
	}

	// query pair contracts
	for _, baseDenom := range baseDenoms {
		res, err := wasmClient.SmartContractState(context.Background(), &wasmtypes.QuerySmartContractStateRequest{
			Address: astroportFactoryContract,
			QueryData: []byte(fmt.Sprintf(`{
			  "pair": {
				"asset_infos": [
				  {
					"native_token": {
					  "denom": "%s"
					}
				  },
				  {
					"native_token": {
					  "denom": "%s"
					}
				  }
				]
			  }
			}`, baseDenom, quoteDenom)),
		})
		if err != nil {
			panic(err)
		}
		fmt.Println("pair contract info:\n", string(res.Data))
	}
}
