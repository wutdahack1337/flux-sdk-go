package main

import (
	"context"
	"fmt"
	wasmtypes "github.com/CosmWasm/wasmd/x/wasm/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
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

	//init wasm client
	wasmClient := wasmtypes.NewQueryClient(cc)

	/*
		astroportFactoryCodeId: 1
		astroportXYKPairCodeId: 2
		astroportRouterCodeId: 3
		cw20BaseCodeId: 4
		astroport factory contract: lux14hj2tavq8fpesdwxxcu44rty3hh90vhujrvcmstl4zr3txmfvw9sm3tpfk
		pair contract info:
		 {"asset_infos":[{"native_token":{"denom":"btc"}},{"native_token":{"denom":"usdt"}}],"contract_addr":"lux1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqhywrts","liquidity_token":"lux1zwv6feuzhy6a9wekh96cd57lsarmqlwxdypdsplw6zhfncqw6ftq0p6czt","pair_type":{"xyk":{}}}
		pair contract info:
		 {"asset_infos":[{"native_token":{"denom":"eth"}},{"native_token":{"denom":"usdt"}}],"contract_addr":"lux1aakfpghcanxtc45gpqlx8j3rq0zcpyf49qmhm9mdjrfx036h4z5sdltq0m","liquidity_token":"lux1xt4ahzz2x8hpkc0tk6ekte9x6crw4w6u0r67cyt3kz9syh24pd7sq5mrz7","pair_type":{"xyk":{}}}
		pair contract info:
		 {"asset_infos":[{"native_token":{"denom":"sol"}},{"native_token":{"denom":"usdt"}}],"contract_addr":"lux18v47nqmhvejx3vc498pantg8vr435xa0rt6x0m6kzhp6yuqmcp8s3z45es","liquidity_token":"lux1ma0g752dl0yujasnfs9yrk6uew7d0a2zrgvg62cfnlfftu2y0egqenprmx","pair_type":{"xyk":{}}}
	*/

	// provide liquidity to pairs
	pairs := map[string]string{
		"lux1nc5tatafv6eyq7llkr2gv50ff9e22mnf70qgjlv737ktmt4eswrqhywrts": "btc/usdt",
		"lux1aakfpghcanxtc45gpqlx8j3rq0zcpyf49qmhm9mdjrfx036h4z5sdltq0m": "eth/usdt",
		"lux18v47nqmhvejx3vc498pantg8vr435xa0rt6x0m6kzhp6yuqmcp8s3z45es": "sol/usdt",
	}
	for contractAddr, ticker := range pairs {
		denoms := strings.Split(ticker, "/")
		amount := int64(10000)
		res, err := chainClient.SyncBroadcastMsg(&wasmtypes.MsgExecuteContract{
			Sender:   senderAddress.String(),
			Contract: contractAddr,
			Msg: []byte(fmt.Sprintf(`{
				"provide_liquidity": {
				  "assets": [
					{
					  "info": {
						"native_token": {
						  "denom": "%s"
						}
					  },
					  "amount": "%d"
					},
					{
					  "info": {
						"native_token": {
						  "denom": "%s"
						}
					  },
					  "amount": "%d"
					}
				  ],
				  "auto_stake": false,
				  "receiver": "%s"
				}
			  }`, denoms[0], amount, denoms[1], amount, senderAddress.String())),
			Funds: sdk.Coins{
				sdk.NewInt64Coin(denoms[0], amount),
				sdk.NewInt64Coin(denoms[1], amount),
			},
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("provided liquidity for pool %s: %s", ticker, res.TxResponse.TxHash))
	}

	// swap token
	for contractAddr, ticker := range pairs {
		denoms := strings.Split(ticker, "/")
		amount := int64(5)
		res, err := chainClient.SyncBroadcastMsg(&wasmtypes.MsgExecuteContract{
			Sender:   senderAddress.String(),
			Contract: contractAddr,
			Msg: []byte(fmt.Sprintf(`{
				"swap": {
				  "offer_asset": {
					"info": {
					  "native_token": {
						"denom": "%s"
					  }
					},
					"amount": "%d"
				  },
				  "max_spread": "0.5",
				  "to": "%s"
				}
			  }`, denoms[1], amount, senderAddress.String())),
			Funds: sdk.Coins{
				sdk.NewInt64Coin(denoms[1], amount),
			},
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("swapped %d %s for %s: %s", amount, denoms[1], denoms[0], res.TxResponse.TxHash))
	}

	// query pools
	for contractAddr, ticker := range pairs {
		res, err := wasmClient.SmartContractState(context.Background(), &wasmtypes.QuerySmartContractStateRequest{
			Address:   contractAddr,
			QueryData: []byte(`{"pool": {}}`),
		})
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("%s pool assets: %s", ticker, string(res.Data)))
	}

}
