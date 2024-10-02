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
	driftPrivKey []byte
	usdtMint     solana.PublicKey
)

type Order struct {
	MarketIndex                        uint16
	AuctionStartPrice, AuctionEndPrice int64
	Quantity                           uint64
	Direction                          drift.PositionDirection
	Oracle                             solana.PublicKey
}

func deposit(
	userClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	depositAmount uint64,
	mint solana.PublicKey,
) {
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, drift.ProgramID)
	if err != nil {
		panic(err)
	}

	spotMarketVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		svm.Uint16ToLeBytes(0),
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

	marketIndex := uint16(0)
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

	svmMsg := svmtypes.ToCosmosMsg([]string{userClient.FromAddress().String()}, 1_000_000, depositTx)
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
	markets []solana.PublicKey, // Note: This injects all markets, which is not optimized => should only inject user's relevant market
	oracles []solana.PublicKey,
) {
	senderAddress := userClient.FromAddress()
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, drift.ProgramID)

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], {0, 0},
	}, drift.ProgramID)

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
	for _, o := range oracles {
		placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
			PublicKey:  o,
			IsWritable: false,
			IsSigner:   false,
		})
	}

	// append all markets
	for _, m := range markets {
		placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
			PublicKey:  m,
			IsWritable: true,
			IsSigner:   false,
		})
	}

	placeOrderTx, err := solana.NewTransactionBuilder().AddInstruction(placeOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svmtypes.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, placeOrderTx)
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
	markets []solana.PublicKey,
	oracles []solana.PublicKey,
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
	for _, o := range oracles {
		fillOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
			PublicKey:  o,
			IsWritable: false,
			IsSigner:   false,
		})
	}

	// append all markets
	for _, m := range markets {
		fillOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
			PublicKey:  m,
			IsWritable: true,
			IsSigner:   false,
		})
	}

	fillOrderTx, err := solana.NewTransactionBuilder().AddInstruction(fillOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svmtypes.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, fillOrderTx)
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

	var programSvmPrivKeyBz []byte
	if err := json.Unmarshal(driftPrivKey, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}
	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	driftProgramId := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())
	drift.SetProgramID(driftProgramId)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	// check and link accounts
	isSvmLinked, userSvmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}
	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := chainClient.LinkSVMAccount(svmKey, math.NewIntFromUint64(1000_000_000_000))
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
		userSvmPubkey = solana.PublicKey(svmKey.PubKey().Bytes())
	} else {
		fmt.Println("sender", senderAddress.String(), "is already linked to svm address:", userSvmPubkey.String())
	}

	// get denom link + deposit
	transferFunds(chainClient)
	denomLink, err := chainClient.GetDenomLink(context.Background(), astromeshtypes.Plane_COSMOS, "usdt", astromeshtypes.Plane_SVM)
	if err != nil {
		panic(err)
	}

	// pre-compute all market addresses
	allMarkets := []solana.PublicKey{}
	for _, spotMarketIndex := range []uint16{0, 1} {
		market, _, err := solana.FindProgramAddress([][]byte{
			[]byte("spot_market"),
			svm.Uint16ToLeBytes(spotMarketIndex),
		}, drift.ProgramID)
		if err != nil {
			panic(err)
		}
		allMarkets = append(allMarkets, market)
	}

	for _, perpMarketIndex := range []uint16{0, 1, 2, 3} {
		market, _, err := solana.FindProgramAddress([][]byte{
			[]byte("perp_market"),
			svm.Uint16ToLeBytes(perpMarketIndex),
		}, drift.ProgramID)
		if err != nil {
			panic(err)
		}
		allMarkets = append(allMarkets, market)
	}

	usdtMintHex := denomLink.DstAddr
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint = solana.PublicKeyFromBytes(usdtMintBz)
	deposit(
		chainClient,
		userSvmPubkey,
		1000_000_000,
		usdtMint,
	)

	fmt.Println("=== user places order ===")
	// get perp markets info
	marketMap := map[uint16]*drift.PerpMarket{}
	allOracles := []solana.PublicKey{}
	for _, marketIndex := range []uint16{0, 1, 2} {
		perpMarketInfo := getDriftPerpMarket(chainClient, marketIndex)
		marketMap[marketIndex] = &perpMarketInfo
		allOracles = append(allOracles, perpMarketInfo.Amm.Oracle)
	}

	driftUser := getDriftUserInfo(chainClient, userSvmPubkey)
	orderId := driftUser.NextOrderId
	marketOrders := []Order{
		{
			MarketIndex:       0,
			AuctionStartPrice: 65020_000_000,
			AuctionEndPrice:   65033_000_000,
			Direction:         drift.PositionDirectionLong,
			Quantity:          500_000,
		},
		{
			MarketIndex:       1,
			AuctionStartPrice: 3001_000_000,
			AuctionEndPrice:   3004_000_000,
			Direction:         drift.PositionDirectionLong,
			Quantity:          500_000,
		},
		{
			MarketIndex:       2,
			AuctionStartPrice: 151_000_000,
			AuctionEndPrice:   151_100_000,
			Direction:         drift.PositionDirectionLong,
			Quantity:          500_000,
		},
	}

	for _, o := range marketOrders {
		placeOrder(
			chainClient,
			userSvmPubkey,
			uint8(orderId),
			uint64(o.AuctionEndPrice), proto.Int64(o.AuctionStartPrice), proto.Int64(o.AuctionEndPrice),
			o.Quantity,
			drift.OrderTypeMarket,
			false,
			drift.PositionDirectionLong,
			1*time.Minute,
			o.MarketIndex,
			svm.Uint8Ptr(10),
			allMarkets,
			allOracles,
		)
		orderId++
	}

	driftUser = getDriftUserInfo(chainClient, userSvmPubkey)
	for _, o := range driftUser.Orders {
		if o.Status == drift.OrderStatusOpen {
			bz, _ := json.MarshalIndent(o, "", "  ")
			fmt.Println("bz:", string(bz))
		}
	}

	fmt.Println("waiting for some seconds for auctions to complete...")
	time.Sleep(11 * time.Second)

	fmt.Println("=== fill all orders against vAMM ===")
	// actually anyone can call this fill_perp_order instruction to fill the order with vAMM
	driftUser = getDriftUserInfo(chainClient, userSvmPubkey)
	for _, o := range driftUser.Orders {
		if o.Status == drift.OrderStatusOpen {
			fillPerpOrder(
				chainClient,
				userSvmPubkey,
				userSvmPubkey,
				o.OrderId,
				o.MarketIndex,
				allMarkets,
				allOracles,
			)
		}
	}

	fmt.Println("user positions:")
	driftUser = getDriftUserInfo(chainClient, userSvmPubkey)
	for _, o := range driftUser.PerpPositions {
		if o.BaseAssetAmount != 0 {
			bz, _ := json.MarshalIndent(o, "", "  ")
			fmt.Println(string(bz))
		}
	}
}
