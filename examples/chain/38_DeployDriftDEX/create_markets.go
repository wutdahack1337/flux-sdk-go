package main

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "embed"

	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/FluxNFTLabs/sdk-go/examples/chain/38_DeployDriftDEX/drift"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/gagliardetto/solana-go"
	"github.com/golang/protobuf/proto"
	"github.com/mr-tron/base58"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	//go:embed artifacts/drift-keypair.json
	driftKeypair []byte

	//go:embed artifacts/pyth-keypair.json
	pythKeypair []byte
)

func uint16ToLeBytes(x uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, x)
	return b
}

func newName(s string) [32]uint8 {
	name := [32]uint8{}
	bz := []byte(s)
	copy(name[:], bz)
	return name
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

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	isSvmLinked, svmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), senderAddress)
	if err != nil {
		panic(err)
	}
	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := chainClient.LinkSVMAccount(svmKey)
		if err != nil {
			panic(err)
		}
		fmt.Println("linked sender to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
		svmPubkey = solana.PublicKey(svmKey.PubKey().Bytes())
	} else {
		fmt.Println("sender is already linked to svm address:", svmPubkey.String())
	}

	// load program, coins id
	var programPythPrivKeBz []byte
	if err := json.Unmarshal(pythKeypair, &programPythPrivKeBz); err != nil {
		panic(err)
	}

	var programSvmPrivKeyBz []byte
	if err := json.Unmarshal(driftKeypair, &programSvmPrivKeyBz); err != nil {
		panic(err)
	}
	programSvmPrivKey := &ed25519.PrivKey{Key: programSvmPrivKeyBz}
	driftProgramId := solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())
	drift.SetProgramID(driftProgramId)

	fmt.Println("drift programId:", driftProgramId.String())

	usdtMintHex := "1c46743a65e0fe89a65a9fe498d8cfa813480358fc1dd4658c6cf842d0560c92"
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint := solana.PublicKeyFromBytes(usdtMintBz)

	btcMintHex := "0811ed5c81d01548aa6cb5177bdeccc835465be58d4fa6b26574f5f7fd258bcd"
	btcMintBz, _ := hex.DecodeString(btcMintHex)
	btcMint := solana.PublicKeyFromBytes(btcMintBz)

	fmt.Println("initialize state and markets")
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, driftProgramId)
	driftSigner, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_signer"),
	}, driftProgramId)

	// admin initialize
	initializeIx := drift.NewInitializeInstruction(
		svmPubkey, state, usdtMint, driftSigner,
		solana.PublicKey(svmtypes.SysVarRent),
		svmtypes.SystemProgramId,
		svmtypes.SplToken2022ProgramId,
	).Build()

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], []byte{0, 0},
	}, driftProgramId)

	userStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), svmPubkey[:],
	}, driftProgramId)

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

	// create market
	// Generate PDA for spot_market
	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for spot_market_vault
	spotMarketUsdtVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for insurance_fund_vault
	insuranceFundUsdtVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("insurance_fund_vault"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	spotMarketBtc, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for spot_market_vault
	spotMarketBtcVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for insurance_fund_vault
	insuranceFundBtcVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("insurance_fund_vault"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	oracleUsdt := svmtypes.SystemProgramId // default pubkey
	oracleBtc := solana.MustPublicKeyFromBase58("3HRnxmtHQrHkooPdFZn5ZQbPTKGvBSyoTi4VVkkoT6u6")

	// Define other necessary public keys
	admin := svmPubkey
	rent := solana.SysVarRentPubkey
	systemProgram := solana.SystemProgramID
	tokenProgram := svmtypes.SplToken2022ProgramId

	optimalUtilization := uint32(8000)
	optimalBorrowRate := uint32(500)
	maxBorrowRate := uint32(1000)
	oracleSourceQuote := drift.OracleSourceQuoteAsset
	oracleSourcePyth := drift.OracleSourcePyth // TODO: Use pyth later

	initialAssetWeight := uint32(10000)
	maintenanceAssetWeight := uint32(10000)
	initialLiabilityWeight := uint32(10000)
	maintenanceLiabilityWeight := uint32(10000)
	imfFactor := uint32(0)
	liquidatorFee := uint32(50)
	ifLiquidationFee := uint32(25)
	activeStatus := true
	assetTier := drift.AssetTierIsolated // TODO: Inspect what is this tier
	scaleInitialAssetWeightStart := uint64(1000000000)
	withdrawGuardThreshold := uint64(500000000)
	orderTickSize := uint64(1000)
	orderStepSize := uint64(100)
	ifTotalFactor := uint32(10)
	nameUsdt := newName("market_usdt")
	nameBtc := newName("market_btc")

	// Create the InitializeSpotMarket instruction
	initializeQuoteSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
		/* Parameters */
		optimalUtilization, optimalBorrowRate, maxBorrowRate,
		oracleSourceQuote, initialAssetWeight, maintenanceAssetWeight,
		initialLiabilityWeight, maintenanceLiabilityWeight, imfFactor,
		liquidatorFee, ifLiquidationFee, activeStatus, assetTier,
		scaleInitialAssetWeightStart, withdrawGuardThreshold,
		orderTickSize, orderStepSize, ifTotalFactor, nameUsdt,
		/* Accounts */
		spotMarketUsdt, usdtMint, spotMarketUsdtVault,
		insuranceFundUsdtVault, driftSigner, state,
		oracleUsdt, admin, rent, systemProgram, tokenProgram,
	).Build()

	initializeBtcSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
		/* Parameters */
		optimalUtilization, optimalBorrowRate, maxBorrowRate,
		oracleSourcePyth, 8000, 9000,
		12000, 11000, 105000,
		liquidatorFee, ifLiquidationFee, activeStatus, assetTier,
		scaleInitialAssetWeightStart, withdrawGuardThreshold,
		orderTickSize, orderStepSize, ifTotalFactor, nameBtc,
		/* Accounts */
		spotMarketBtc, btcMint, spotMarketBtcVault,
		insuranceFundBtcVault, driftSigner, state,
		oracleBtc, admin, rent, systemProgram, tokenProgram,
	).Build()

	initializeTx, err := solana.NewTransactionBuilder().
		AddInstruction(initializeIx).
		AddInstruction(initializeUserStatsIx).
		AddInstruction(initializeUserIx).
		AddInstruction(initializeQuoteSpotMarketIx).
		AddInstruction(initializeBtcSpotMarketIx).Build()
	if err != nil {
		panic(err)
	}

	marketExist := true
	_, err = chainClient.GetSvmAccount(context.Background(), spotMarketBtc.String())
	if err != nil && !strings.Contains(err.Error(), "not existed") {
		panic(err)
	}

	if err != nil {
		marketExist = false
	}

	if !marketExist {
		svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, initializeTx)
		res, err := chainClient.SyncBroadcastMsg(svmMsg)
		if err != nil {
			panic(err)
		}

		fmt.Println("== init account and create market ==")
		fmt.Println("tx hash:", res.TxResponse.TxHash)
		fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	} else {
		fmt.Println("account and market already initialized")
	}

	depositAmount := 6500_000_00 // 0.1 BTC
	userTokenAccount, _, err := solana.FindProgramAddress([][]byte{
		svmPubkey[:], svmtypes.SplToken2022ProgramId[:], usdtMint[:],
	}, svmtypes.AssociatedTokenProgramId)
	if err != nil {
		panic(err)
	}
	depositIxBuilder := drift.NewDepositInstruction(
		0, uint64(depositAmount), false, state, user, userStats, svmPubkey, spotMarketUsdtVault, userTokenAccount, svmtypes.SplToken2022ProgramId,
	)

	depositIxBuilder.Append(&solana.AccountMeta{
		PublicKey:  spotMarketUsdt,
		IsWritable: true,
		IsSigner:   false,
	})
	depositIx := depositIxBuilder.Build()

	// place spot market order
	// Define the OrderParams with default or specified values
	unixNow := time.Now().Unix()
	auctionDur := uint8(10)
	orderParams := drift.OrderParams{
		OrderType:         drift.OrderTypeLimit,
		MarketType:        drift.MarketTypeSpot,
		Direction:         drift.PositionDirectionLong,
		UserOrderId:       1,
		BaseAssetAmount:   1000000,
		Price:             65_000_000_000,
		MarketIndex:       1,
		ReduceOnly:        false,
		PostOnly:          drift.PostOnlyParamNone,
		ImmediateOrCancel: false,
		MaxTs:             &unixNow,
		TriggerPrice:      proto.Uint64(0),
		TriggerCondition:  drift.OrderTriggerConditionAbove,
		OraclePriceOffset: proto.Int32(0),
		AuctionDuration:   &auctionDur,
		AuctionStartPrice: proto.Int64(64_000_000_000),
		AuctionEndPrice:   proto.Int64(66_000_000_000),
	}

	// Create the PlaceOrder instruction
	fmt.Println("placing order...")
	placeOrderIx := drift.NewPlaceSpotOrderInstruction(
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

	// append all spot market
	placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  spotMarketUsdt,
		IsWritable: false,
		IsSigner:   false,
	})
	placeOrderIx.AccountMetaSlice.Append(&solana.AccountMeta{
		PublicKey:  spotMarketBtc,
		IsWritable: false,
		IsSigner:   false,
	})

	placeOrderTx, err := solana.NewTransactionBuilder().
		AddInstruction(depositIx).
		AddInstruction(placeOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, placeOrderTx)
	res, err := chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("create order:", res)
}
