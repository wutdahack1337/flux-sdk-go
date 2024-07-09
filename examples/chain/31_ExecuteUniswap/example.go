package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"

	sdkmath "cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	evmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/evm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/gogoproto/proto"
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

type ModifyLiquidityParams struct {
	TickLower      *big.Int
	TickUpper      *big.Int
	LiquidityDelta *big.Int
	Salt           [32]byte
}

type SwapParams struct {
	ZeroForOne        bool     // swap direction currency0 -> currency1
	AmountSpecified   *big.Int // positive: output amount, negative: input amount
	SqrtPriceLimitX96 *big.Int // equivalent slippage limit
}

func signedBigIntFromBytes(b []byte) *big.Int {
	res := new(big.Int).SetBytes(b)
	// first bit is set, then it's negative
	if b[0]&0x80 != 0 {
		bitCount := len(b) * 8
		complement := new(big.Int).Lsh(big.NewInt(1), uint(bitCount))
		res = new(big.Int).Sub(res, complement)
	}

	return res
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

	// init evm client
	astromeshClient := astromeshtypes.NewQueryClient(cc)

	// read bytecode
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	bz, err := os.ReadFile(dir + "/examples/chain/28_DeployUniswap/PoolManager.json")
	if err != nil {
		panic(err)
	}
	var compData0 map[string]interface{}
	err = json.Unmarshal(bz, &compData0)
	abiBz, err := json.Marshal(compData0["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	poolManagerABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	var compData map[string]interface{}
	bz, err = os.ReadFile(dir + "/examples/chain/28_DeployUniswap/PoolActions.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bz, &compData)
	abiBz, err = json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	poolActionsABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	bz, err = os.ReadFile(dir + "/examples/chain/29_ExecuteUniswap/erc20.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(bz, &compData)
	abiBz, err = json.Marshal(compData["abi"].([]interface{}))
	if err != nil {
		panic(err)
	}
	erc20ABI, err := abi.JSON(strings.NewReader(string(abiBz)))
	if err != nil {
		panic(err)
	}

	// parse contract addr
	PoolManagerContractAddr, err := hex.DecodeString("07aa076883658b7ed99d25b1e6685808372c8fe2")
	if err != nil {
		panic(err)
	}
	PoolActionsContractAddr, err := hex.DecodeString("e2f81b30e1d47dffdbb6ab41ec5f0572705b026d")
	if err != nil {
		panic(err)
	}

	// tick spacing: 2^0 - 2^15
	tokens := []string{"btc", "eth", "sol", "usdt"}
	tokenContracts := map[string]string{}
	fmt.Println("===========Transfer tokens to EVM Plane to create ERC20 contracts===========")

	for _, token := range tokens {
		//perform astrotransfer to evm planes for cosmos tokens
		amount, _ := sdkmath.NewIntFromString("100000000000000000000") // 100 * 10^18
		_, err := chainClient.SyncBroadcastMsg(&astromeshtypes.MsgAstroTransfer{
			Sender:   senderAddress.String(),
			Receiver: senderAddress.String(),
			SrcPlane: astromeshtypes.Plane_COSMOS,
			DstPlane: astromeshtypes.Plane_EVM,
			Coin: sdk.Coin{
				Denom:  token,
				Amount: amount,
			},
		})
		if err != nil {
			panic(err)
		}
		denomLink, err := astromeshClient.DenomLink(context.Background(), &astromeshtypes.QueryDenomLinkRequest{
			SrcPlane: astromeshtypes.Plane_COSMOS,
			DstPlane: astromeshtypes.Plane_EVM,
			SrcAddr:  token,
		})
		if err != nil {
			panic(err)
		}
		tokenContracts[token] = denomLink.DstAddr
		fmt.Println(fmt.Sprintf("%s: %s, decimals %d", token, denomLink.DstAddr, denomLink.DstDecimals))

		// approve PoolActions contract to spend token
		allowance := big.NewInt(100000000000000000)
		calldata, err := erc20ABI.Pack("approve", ethcommon.Address(PoolActionsContractAddr), allowance)
		if err != nil {
			panic(err)
		}
		msg := &evmtypes.MsgExecuteContract{
			Sender:          senderAddress.String(),
			ContractAddress: ethcommon.Hex2Bytes(denomLink.DstAddr),
			Calldata:        calldata,
		}
		_, err = chainClient.SyncBroadcastMsg(msg)
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("approved pool actions contract to spend %d%s", allowance.Int64(), token))
	}

	// initialize pairs
	prices := map[string]float64{
		"btc": 69000,
		"eth": 3900,
		"sol": 185,
	}
	for denom, contractAddr := range tokenContracts {
		if denom == "usdt" {
			continue
		}

		// uniswap make sure only a single pair of 2 addresses can exist by comparing token addresses
		usdtAddr := tokenContracts["usdt"]
		currencies := []string{usdtAddr, contractAddr}
		sort.Strings(currencies)

		// get correct price ratio base on contract order
		price := prices[denom]
		denom0 := denom
		denom1 := "usdt"
		if usdtAddr == currencies[0] {
			price = 1 / price
			denom0 = "usdt"
			denom1 = denom
		}

		fmt.Println(fmt.Sprintf("===========Pool %s-%s===========", denom0, denom1))

		tickSpacing := int64(60)
		pairKey := &PairKey{
			Currency0:   ethcommon.HexToAddress(currencies[0]),
			Currency1:   ethcommon.HexToAddress(currencies[1]),
			Fee:         big.NewInt(3000),
			TickSpacing: big.NewInt(tickSpacing),
			Hooks:       ethcommon.HexToAddress("0x"),
		}

		sqrtPriceX96Int := computeSqrtPriceX96Int(price)
		calldata, err := poolManagerABI.Pack("initialize", pairKey, sqrtPriceX96Int, []byte{})
		if err != nil {
			panic(err)
		}

		msg := &evmtypes.MsgExecuteContract{
			Sender:          senderAddress.String(),
			ContractAddress: PoolManagerContractAddr,
			Calldata:        calldata,
		}

		res, err := chainClient.SyncBroadcastMsg(msg)
		if err != nil {
			panic(err)
		}

		// unpack execute return data
		var msgData sdk.TxMsgData
		var executeRes evmtypes.MsgExecuteContractResponse
		resBz := ethcommon.Hex2Bytes(res.TxResponse.Data)
		proto.Unmarshal(resBz, &msgData)
		proto.Unmarshal(msgData.MsgResponses[0].Value, &executeRes)
		initializeRes, err := poolManagerABI.Unpack("initialize", executeRes.Output)
		if err != nil {
			panic(err)
		}
		currentTick := initializeRes[0].(*big.Int)
		fmt.Println(fmt.Sprintf("initialized tick %d: %s", currentTick.Uint64(), res.TxResponse.TxHash))

		// provide liquidity in +-20% range
		lowerPrice := float64(price) * 0.8
		upperPrice := float64(price) * 1.2
		modifyLiquidityParams := &ModifyLiquidityParams{
			TickLower:      computeTick(lowerPrice, tickSpacing),
			TickUpper:      computeTick(upperPrice, tickSpacing),
			LiquidityDelta: big.NewInt(1000000000),
			Salt:           [32]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 0, 1, 2},
		}
		calldata, err = poolActionsABI.Pack("actionModifyLiquidity", pairKey, modifyLiquidityParams, []byte{})
		if err != nil {
			panic(err)
		}
		msg = &evmtypes.MsgExecuteContract{
			Sender:          senderAddress.String(),
			ContractAddress: PoolActionsContractAddr,
			Calldata:        calldata,
		}
		res, err = chainClient.SyncBroadcastMsg(msg)
		if err != nil {
			panic(err)
		}
		fmt.Println(fmt.Sprintf("liquidity added: %s", res.TxResponse.TxHash))

		// perform swap on pool
		swapParams := &SwapParams{
			ZeroForOne:        true,
			AmountSpecified:   big.NewInt(-5000),
			SqrtPriceLimitX96: computeSqrtPriceX96Int(lowerPrice),
		}

		calldata, err = poolActionsABI.Pack("actionSwap", pairKey, swapParams, []byte{})
		if err != nil {
			panic(err)
		}

		msg = &evmtypes.MsgExecuteContract{
			Sender:          senderAddress.String(),
			ContractAddress: PoolActionsContractAddr,
			Calldata:        calldata,
		}
		res, err = chainClient.SyncBroadcastMsg(msg)
		if err != nil {
			panic(err)
		}
		fmt.Println("swapped:", res.TxResponse.TxHash)

		hexResp, err := hex.DecodeString(res.TxResponse.Data)
		if err != nil {
			panic(fmt.Errorf("decode response hex err: %w", err))
		}

		var txData sdk.TxMsgData
		if err := txData.Unmarshal(hexResp); err != nil {
			panic(err)
		}

		var r evmtypes.MsgExecuteContractResponse
		if err := r.Unmarshal(txData.MsgResponses[0].Value); err != nil {
			panic(fmt.Errorf("unmarshal evm execute contract err: %w", err))
		}

		// we know for sure it will return an int256
		if len(r.Output) < 32 {
			panic(fmt.Errorf("swap output must have 32 bytes: %v", r.Output))
		}

		deltaCurrency0, deltaCurrency1 := r.Output[:16], r.Output[16:32]
		deltaCurrency0Int, deltaCurrency1Int := new(big.Int).Abs(signedBigIntFromBytes(deltaCurrency0)), new(big.Int).Abs(signedBigIntFromBytes(deltaCurrency1))
		// swap amount for correct display if we swap 1 => 0
		if !swapParams.ZeroForOne {
			deltaCurrency0Int, deltaCurrency1Int = deltaCurrency1Int, deltaCurrency0Int
		}

		fmt.Println("swapped", deltaCurrency0Int.String(), denom0, "for", deltaCurrency1Int.String(), denom1)
	}

}

func computeSqrtPriceX96Int(p float64) *big.Int {
	sqrtPrice := new(big.Float).Sqrt(big.NewFloat(p))
	factor := new(big.Float).SetInt(new(big.Int).Lsh(big.NewInt(1), 96))
	sqrtPrice.Mul(sqrtPrice, factor)
	sqrtPriceX96Int := new(big.Int)
	price, _ := sqrtPrice.Int(sqrtPriceX96Int)
	return price
}

func logBigFloat(x *big.Float) *big.Float {
	f64, _ := x.Float64()
	return big.NewFloat(math.Log(f64))
}

func computeTick(price float64, spacing int64) *big.Int {
	factor := big.NewFloat(1.0001)
	priceBig := big.NewFloat(price)
	logPrice := logBigFloat(priceBig)
	logFactor := logBigFloat(factor)
	floatTick := new(big.Float).Quo(logPrice, logFactor)
	spc := big.NewInt(spacing)

	// round tick to closest spacing
	intTick := new(big.Int)
	floatTick.Int(intTick)
	roundedTick := new(big.Int).Mul(new(big.Int).Quo(intTick, spc), spc)
	return roundedTick
}
