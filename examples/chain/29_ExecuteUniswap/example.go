package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
)

type PairKey struct {
	Currency0   ethcommon.Address // base denom addr
	Currency1   ethcommon.Address // quote denom addr
	Fee         *big.Int          // 3000 -> 0.3%
	TickSpacing *big.Int
	Hooks       ethcommon.Address // hook contract addr
}

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
	cc, err := grpc.Dial("localhost:9900", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	// init evm client
	evmClient := evmtypes.NewQueryClient(cc)

	// read bytecode
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	bz, err := os.ReadFile(dir + "/examples/chain/28_DeployUniswap/PoolManager.json")
	if err != nil {
		panic(err)
	}

	var compData map[string]interface{}
	err = json.Unmarshal(bz, &compData)
	abiBz, err := json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	contractABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	// parse contract addr
	contractAddr, err := hex.DecodeString("eef74ab95099c8d1ad8de02ba6bdab9cbc9dbf93")
	if err != nil {
		panic(err)
	}

	// query some data
	calldata, err := contractABI.Pack("MAX_TICK_SPACING")
	if err != nil {
		panic(err)
	}
	queryRes, err := evmClient.ContractQuery(context.Background(), &evmtypes.ContractQueryRequest{
		Address:  "eef74ab95099c8d1ad8de02ba6bdab9cbc9dbf93",
		Calldata: calldata,
	})
	if err != nil {
		panic(err)
	}
	queryOutput, err := contractABI.Unpack("MAX_TICK_SPACING", queryRes.Output)
	if err != nil {
		panic(err)
	}
	fmt.Println("query MAX_TICK_SPACING:", queryOutput)

	// prepare tx msg
	pairKey := &PairKey{
		Currency0:   ethcommon.Address{},
		Currency1:   ethcommon.Address{},
		Fee:         big.NewInt(3000),
		TickSpacing: big.NewInt(60),
		Hooks:       ethcommon.Address{},
	}

	// 1 btc = 69000 usdt
	sqrtPriceX96Int, _ := new(big.Int).SetString("5424589537444978962029036245656805120289623703644", 10)
	hookData := []byte{}
	calldata, err = contractABI.Pack("initialize", pairKey, sqrtPriceX96Int, hookData)
	if err != nil {
		panic(err)
	}

	msg := &evmtypes.MsgExecuteContract{
		Sender:          senderAddress.String(),
		ContractAddress: contractAddr,
		Calldata:        calldata,
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
	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var dcr evmtypes.MsgDeployContractResponse
	if err := dcr.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		panic(err)
	}
	fmt.Println("contract address:", hex.EncodeToString(dcr.ContractAddress))
}
