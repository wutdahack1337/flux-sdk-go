package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"math/big"
	"os"
	"strings"

	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
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
		"user2",
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

	// deploy flux processor contract
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	bz, err := os.ReadFile(dir + "/examples/chain/16_MsgDeployEVMContract/fluxProcessorData.json")
	if err != nil {
		panic(err)
	}

	var compData map[string]interface{}
	err = json.Unmarshal(bz, &compData)
	if err != nil {
		panic(err)
	}
	bytecode, err := hex.DecodeString(compData["bytecode"].(map[string]interface{})["object"].(string))
	if err != nil {
		panic(err)
	}

	// prepare tx msg
	msg := &evmtypes.MsgDeployContract{
		Sender:   senderAddress.String(),
		Bytecode: bytecode,
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

	// decode result to get contract address
	var txData1 sdk.TxMsgData
	if err := txData1.Unmarshal(hexResp); err != nil {
		panic(err)
	}
	var dcr evmtypes.MsgDeployContractResponse
	if err := dcr.Unmarshal(txData1.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	fmt.Println("contract owner:", senderAddress.String())
	fmt.Println("flux processor contract address:", hex.EncodeToString(dcr.ContractAddress))

	// deploy a + b = c contract
	bz, err = os.ReadFile(dir + "/examples/chain/16_MsgDeployEVMContract/addData.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bz, &compData)
	if err != nil {
		panic(err)
	}
	bytecode, err = hex.DecodeString(compData["bytecode"].(map[string]interface{})["object"].(string))
	if err != nil {
		panic(err)
	}
	abiBz, err := json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	abi, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}
	callData, err := abi.Pack("", ethcommon.Address(dcr.ContractAddress[:]), big.NewInt(3), big.NewInt(4))
	if err != nil {
		panic(err)
	}

	// prepare tx msg
	msg = &evmtypes.MsgDeployContract{
		Sender:   senderAddress.String(),
		Bytecode: bytecode,
		Calldata: callData,
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

	// decode result to get contract address
	var txData2 sdk.TxMsgData
	if err = txData2.Unmarshal(hexResp); err != nil {
		panic(err)
	}
	if err = dcr.Unmarshal(txData2.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	fmt.Println("contract owner:", senderAddress.String())
	fmt.Println("add contract address:", hex.EncodeToString(dcr.ContractAddress))
}
