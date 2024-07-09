package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"math/big"
	"os"
	"strings"

	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
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

	// read bytecode
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	poolManagerBz, err := os.ReadFile(dir + "/examples/chain/28_DeployUniswap/PoolManager.json")
	if err != nil {
		panic(err)
	}
	poolActionsBz, err := os.ReadFile(dir + "/examples/chain/28_DeployUniswap/PoolActions.json")
	if err != nil {
		panic(err)
	}

	var compData map[string]interface{}
	err = json.Unmarshal(poolManagerBz, &compData)
	if err != nil {
		panic(err)
	}
	poolManagerBytecode, err := hex.DecodeString(compData["bytecode"].(map[string]interface{})["object"].(string))
	if err != nil {
		panic(err)
	}
	abiBz, err := json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	poolManagerABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(poolActionsBz, &compData)
	if err != nil {
		panic(err)
	}
	PoolActionsBytecode, err := hex.DecodeString(compData["bytecode"].(map[string]interface{})["object"].(string))
	if err != nil {
		panic(err)
	}
	abiBz, err = json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	PoolActionsABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	// deploy pool manager
	calldata, err := poolManagerABI.Pack("", big.NewInt(4000000))
	if err != nil {
		panic(err)
	}

	msg := &evmtypes.MsgDeployContract{
		Sender:   senderAddress.String(),
		Bytecode: poolManagerBytecode,
		Calldata: calldata,
	}

	txResp, err := chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)

	hexResp, err := hex.DecodeString(txResp.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var dcr evmtypes.MsgDeployContractResponse
	if err := dcr.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	poolManagerAddr := dcr.ContractAddress
	fmt.Println("pool manager address:", hex.EncodeToString(poolManagerAddr))

	// deploy pool actions
	calldata, err = PoolActionsABI.Pack("", ethcommon.Address(poolManagerAddr))
	if err != nil {
		panic(err)
	}
	msg = &evmtypes.MsgDeployContract{
		Sender:   senderAddress.String(),
		Bytecode: PoolActionsBytecode,
		Calldata: calldata,
	}

	txResp, err = chainClient.SyncBroadcastMsg(msg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)

	hexResp, err = hex.DecodeString(txResp.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	txData = sdk.TxMsgData{}
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}
	if err := dcr.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	poolActionsAddr := dcr.ContractAddress
	fmt.Println("pool actions address:", hex.EncodeToString(poolActionsAddr))
}
