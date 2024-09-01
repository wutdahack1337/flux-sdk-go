package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "embed"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/FluxNFTLabs/sdk-go/client/svm/drift"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/golang/protobuf/proto"
	"github.com/mr-tron/base58"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	driftPrivKey        []byte
	oracleBtc           = solana.MustPublicKeyFromBase58("3HRnxmtHQrHkooPdFZn5ZQbPTKGvBSyoTi4VVkkoT6u6")
	usdtMint, btcMint   solana.PublicKey
	usdtSpotMarketIndex = uint16(0)
	btcMarketIndex      = uint16(0)
)

func deposit(
	userClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	depositAmount uint64,
	mint solana.PublicKey,
	marketIndex uint16,
) {
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	spotMarketVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		svm.Uint16ToLeBytes(marketIndex),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	userTokenAccount, _, err := solana.FindProgramAddress([][]byte{
		svmPubkey[:], svmtypes.SplToken2022ProgramId[:], mint[:],
	}, svmtypes.AssociatedTokenProgramId)
	if err != nil {
		panic(err)
	}

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], {0, 0},
	}, drift.ProgramID)

	userStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), svmPubkey[:],
	}, drift.ProgramID)

	spotMarket, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		svm.Uint16ToLeBytes(marketIndex),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	initializeUserStatsIx := drift.NewInitializeUserStatsInstruction(
		userStats, state, svmPubkey, svmPubkey, solana.PublicKey(svmtypes.SysVarRent), svmtypes.SystemProgramId,
	).Build()

	initializeUserIx := drift.NewInitializeUserInstruction(
		0,
		[32]uint8{},
		user,
		userStats,
		state,
		svmPubkey,
		svmPubkey,
		solana.PublicKey(svmtypes.SysVarRent), svmtypes.SystemProgramId,
	).Build()

	depositIxBuilder := drift.NewDepositInstruction(
		marketIndex, uint64(depositAmount), false, state, user, userStats, svmPubkey, spotMarketVault, userTokenAccount, svmtypes.SplToken2022ProgramId,
	)

	if marketIndex > 0 {
		depositIxBuilder.Append(&solana.AccountMeta{
			PublicKey:  oracleBtc,
			IsWritable: true,
			IsSigner:   false,
		})
	}

	depositIxBuilder.Append(&solana.AccountMeta{
		PublicKey:  spotMarket,
		IsWritable: true,
		IsSigner:   false,
	})
	depositIx := depositIxBuilder.Build()

	accountExist := true
	_, err = userClient.GetSvmAccount(context.Background(), userStats.String())
	if err != nil && !strings.Contains(err.Error(), "not existed") {
		panic(err)
	}

	if err != nil {
		accountExist = false
	}

	depositTxBuilder := solana.NewTransactionBuilder()
	if !accountExist {
		depositTxBuilder = depositTxBuilder.AddInstruction(initializeUserStatsIx).AddInstruction(initializeUserIx)
	}

	depositTxBuilder = depositTxBuilder.AddInstruction(depositIx)
	depositTx, err := depositTxBuilder.Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{userClient.FromAddress().String()}, 1_000_000, depositTx)
	res, err := userClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func placeOrder(
	userClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	orderId uint8,
	price uint64, auctionStartPrice, auctionEndPrice *int64,
	baseAssetAmount uint64,
	orderType drift.OrderType,
	immediateOrCancel bool,
	direction drift.PositionDirection,
	expireDuration time.Duration,
	marketIndex uint16,
	auctionDur *uint8,
) {
	senderAddress := userClient.FromAddress()
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, drift.ProgramID)

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], {0, 0},
	}, drift.ProgramID)

	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		svm.Uint16ToLeBytes(0),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	perpMarket, _, err := solana.FindProgramAddress([][]byte{
		[]byte("perp_market"),
		svm.Uint16ToLeBytes(marketIndex),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	// Define the OrderParams with default or specified values
	unixExpireTime := time.Now().Add(expireDuration).Unix()
	orderParams := drift.OrderParams{
		OrderType:         orderType,
		MarketType:        drift.MarketTypePerp,
		Direction:         direction,
		UserOrderId:       orderId,
		BaseAssetAmount:   baseAssetAmount,
		Price:             price,
		MarketIndex:       marketIndex,
		ReduceOnly:        false,
		PostOnly:          drift.PostOnlyParamNone,
		ImmediateOrCancel: immediateOrCancel,
		MaxTs:             &unixExpireTime,
		TriggerPrice:      proto.Uint64(0),
		TriggerCondition:  drift.OrderTriggerConditionAbove,
		OraclePriceOffset: proto.Int32(0),
		AuctionDuration:   auctionDur,
		AuctionStartPrice: auctionStartPrice,
		AuctionEndPrice:   auctionEndPrice,
	}

	// Create the PlaceOrder instruction
	placeOrderIx := drift.NewPlacePerpOrderInstruction(
		orderParams,
		state,
		user,
		svmPubkey,
	)

	// append all oracles
	placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  oracleBtc,
		IsWritable: false,
		IsSigner:   false,
	})

	// append all  markets
	placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  spotMarketUsdt,
		IsWritable: true,
		IsSigner:   false,
	})

	placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  perpMarket,
		IsWritable: true,
		IsSigner:   false,
	})

	placeOrderTx, err := solana.NewTransactionBuilder().AddInstruction(placeOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, placeOrderTx)
	res, err := userClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func fillPerpOrder(
	userClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	takerPubkey solana.PublicKey,
	takerOrderId uint32,
	marketIndex uint16,
) {
	senderAddress := userClient.FromAddress()
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, drift.ProgramID)

	filler, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], {0, 0},
	}, drift.ProgramID)

	fillerStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), svmPubkey[:],
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	takerUser, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), takerPubkey[:], {0, 0},
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	takerUserStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), takerPubkey[:],
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		svm.Uint16ToLeBytes(0),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	perpMarketBtc, _, err := solana.FindProgramAddress([][]byte{
		[]byte("perp_market"),
		svm.Uint16ToLeBytes(marketIndex),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	// Create the PlaceOrder instruction
	fillOrderIx := drift.NewFillPerpOrderInstruction(
		takerOrderId,
		0,
		state, svmPubkey, filler, fillerStats,
		takerUser, takerUserStats,
	)

	// fill against vAMM => no need maker order
	fillOrderIx.MakerOrderId = nil

	// append all oracles
	fillOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  oracleBtc,
		IsWritable: false,
		IsSigner:   false,
	})

	// append all markets
	fillOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  spotMarketUsdt,
		IsWritable: true,
		IsSigner:   false,
	})

	fillOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  perpMarketBtc,
		IsWritable: true,
		IsSigner:   false,
	})

	fillOrderTx, err := solana.NewTransactionBuilder().AddInstruction(fillOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, fillOrderTx)
	res, err := userClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func transferFunds(
	chainClient chainclient.ChainClient,
) {
	senderAddress := chainClient.FromAddress()
	msg1 := &astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin: sdk.Coin{
			Denom:  "btc",
			Amount: math.NewIntFromUint64(10000000000),
		},
	}
	msg2 := &astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin: sdk.Coin{
			Denom:  "usdt",
			Amount: math.NewIntFromUint64(100000000000),
		},
	}

	txResp, err := chainClient.SyncBroadcastMsg(msg1, msg2)
	if err != nil {
		panic(err)
	}
	fmt.Println("=== astro transfer to prepare svm funds ===")
	fmt.Println("resp:", txResp.TxResponse.TxHash)
	fmt.Println("gas used/want:", txResp.TxResponse.GasUsed, "/", txResp.TxResponse.GasWanted)
}

func getDriftUserInfo(chainClient chainclient.ChainClient, accPubkey solana.PublicKey) drift.User {
	userAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), accPubkey[:], {0, 0},
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	acc, err := chainClient.GetSvmAccount(context.Background(), userAccount.String())
	if err != nil {
		panic(err)
	}

	var user drift.User
	err = user.UnmarshalWithDecoder(bin.NewBinDecoder(acc.Account.Data))
	if err != nil {
		panic(err)
	}

	return user
}

func getDriftPerpMarket(chainClient chainclient.ChainClient, idx uint16) drift.PerpMarket {
	perpMarket, _, err := solana.FindProgramAddress([][]byte{
		[]byte("perp_market"),
		svm.Uint16ToLeBytes(idx),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	acc, err := chainClient.GetSvmAccount(context.Background(), perpMarket.String())
	if err != nil {
		panic(err)
	}

	var pm drift.PerpMarket
	err = pm.UnmarshalWithDecoder(bin.NewBinDecoder(acc.Account.Data))
	if err != nil {
		panic(err)
	}

	return pm
}

func main() {
	network := common.LoadNetwork("local", "")
	kr, err := keyring.New(
		"fluxd",
		"file",
		os.Getenv("HOME")+"/.fluxd",
		strings.NewReader("12345678"),
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

	// load artifacts
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	driftPrivKey, err = os.ReadFile(dir + "/examples/chain/38_DeployDriftDEX/artifacts/drift-keypair.json")
	if err != nil {
		panic(err)
	}

	// init chain client
	userClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	// check and link accounts
	isSvmLinked, userSvmPubkey, err := userClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}
	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := userClient.LinkSVMAccount(svmKey, math.NewIntFromUint64(1000_000_000_000))
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
		userSvmPubkey = solana.PublicKey(svmKey.PubKey().Bytes())
	} else {
		fmt.Println("sender", senderAddress.String(), "is already linked to svm address:", userSvmPubkey.String())
	}

	transferFunds(userClient)

	denomHexMap := map[string]string{}
	for _, denom := range []string{"btc", "usdt"} {
		denomLink, err := userClient.GetDenomLink(context.Background(), astromeshtypes.Plane_COSMOS, denom, astromeshtypes.Plane_SVM)
		if err != nil {
			panic(err)
		}

		denomHexMap[denom] = denomLink.DstAddr
	}

	usdtMintHex := denomHexMap["usdt"]
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint = solana.PublicKeyFromBytes(usdtMintBz)

	btcMintHex := denomHexMap["btc"]
	btcMintBz, _ := hex.DecodeString(btcMintHex)
	btcMint = solana.PublicKeyFromBytes(btcMintBz)

	var programSvmPrivKeyBz []byte
	if err := json.Unmarshal(driftPrivKey, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}
	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	driftProgramId := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())
	drift.SetProgramID(driftProgramId)
	deposit(
		userClient,
		userSvmPubkey,
		1000_000_000,
		usdtMint,
		usdtSpotMarketIndex,
	)

	driftUser := getDriftUserInfo(userClient, userSvmPubkey)
	orderId := driftUser.NextOrderId

	perp := getDriftPerpMarket(userClient, 0)
	bz, _ := json.Marshal(perp)
	fmt.Println("perp:", string(bz))

	fmt.Println("=== user places order ===")
	placeOrder(
		userClient,
		userSvmPubkey,
		uint8(orderId),
		65050_000_000, proto.Int64(65020_000_000), proto.Int64(65035_000_000),
		500_000,
		drift.OrderTypeMarket,
		false,
		drift.PositionDirectionLong,
		30*time.Second,
		btcMarketIndex,
		svm.Uint8Ptr(0),
	)
	fmt.Println("user order_id:", orderId)
	fmt.Println("waiting for some seconds for auction to complete...")
	time.Sleep(11 * time.Second)

	fmt.Printf("=== fill orders %d against vAMM ===\n", orderId)
	// actually anyone can call this fill_perp_order instruction to fill the order
	// to make the code simpler, it uses userClient
	fillPerpOrder(
		userClient,
		userSvmPubkey,
		userSvmPubkey,
		orderId,
		btcMarketIndex,
	)

	fmt.Println("user positions:")
	for _, o := range driftUser.PerpPositions {
		if o.BaseAssetAmount != 0 {
			bz, _ := json.MarshalIndent(o, "", "  ")
			fmt.Println(string(bz))
		}
	}
}
