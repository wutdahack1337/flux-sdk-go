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
)

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

	userStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), svmPubkey[:],
	}, driftProgramId)

	initializeStateIx := drift.NewInitializeUserStatsInstruction(
		userStats, state, svmPubkey, svmPubkey, solana.PublicKey(svmtypes.SysVarRent), svmtypes.SystemProgramId,
	).Build()

	initializeUserIx := drift.NewInitializeUserInstruction(
		0,
		[32]uint8{},
		svmPubkey,
		userStats,
		state,
		svmPubkey,
		svmPubkey,
		solana.PublicKey(svmtypes.SysVarRent), svmtypes.SystemProgramId,
	).Build()

	// create market
	// Generate PDA for spot_market
	spotMarket, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		state.Bytes(),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for spot_market_vault
	spotMarketVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		state.Bytes(),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// Generate PDA for insurance_fund_vault
	insuranceFundVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("insurance_fund_vault"),
		state.Bytes(),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	oracle := svmtypes.SystemProgramId
	// Define other necessary public keys
	admin := svmPubkey
	rent := solana.SysVarRentPubkey
	systemProgram := solana.SystemProgramID
	tokenProgram := solana.TokenProgramID

	optimalUtilization := uint32(8000)
	optimalBorrowRate := uint32(500)
	maxBorrowRate := uint32(1000)
	oracleSource := drift.OracleSourceQuoteAsset // TODO: Use pyth later
	initialAssetWeight := uint32(10000)
	maintenanceAssetWeight := uint32(8000)
	initialLiabilityWeight := uint32(10000)
	maintenanceLiabilityWeight := uint32(12000)
	imfFactor := uint32(100)
	liquidatorFee := uint32(50)
	ifLiquidationFee := uint32(25)
	activeStatus := true
	assetTier := drift.AssetTierIsolated // TODO: Inspect what is this tier
	scaleInitialAssetWeightStart := uint64(1000000000)
	withdrawGuardThreshold := uint64(500000000)
	orderTickSize := uint64(1000)
	orderStepSize := uint64(100)
	ifTotalFactor := uint32(10)
	name := [32]uint8{'M', 'a', 'r', 'k', 'e', 't', '1'}

	// Create the InitializeSpotMarket instruction
	initializeSpotMarketIx := drift.NewInitializeSpotMarketInstruction(
		/* Parameters */
		optimalUtilization, optimalBorrowRate, maxBorrowRate,
		oracleSource, initialAssetWeight, maintenanceAssetWeight,
		initialLiabilityWeight, maintenanceLiabilityWeight, imfFactor,
		liquidatorFee, ifLiquidationFee, activeStatus, assetTier,
		scaleInitialAssetWeightStart, withdrawGuardThreshold,
		orderTickSize, orderStepSize, ifTotalFactor, name,
		/* Accounts */
		spotMarket, btcMint, spotMarketVault,
		insuranceFundVault, driftSigner, state,
		oracle, admin, rent, systemProgram, tokenProgram,
	).Build()

	initializeTx, err := solana.NewTransactionBuilder().
		AddInstruction(initializeIx).
		AddInstruction(initializeStateIx).
		AddInstruction(initializeUserIx).
		AddInstruction(initializeSpotMarketIx).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, initializeTx)
	res, err := chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}

	fmt.Println("initialize account:", res)
	// place spot market order
	// Define the OrderParams with default or specified values
	unixNow := time.Now().Unix()
	auctionDur := uint8(8)
	orderParams := drift.OrderParams{
		OrderType:         drift.OrderTypeLimit,             // Example value
		MarketType:        drift.MarketTypeSpot,             // Example value
		Direction:         drift.PositionDirectionLong,      // Example value
		UserOrderId:       1,                                // Example user order ID
		BaseAssetAmount:   1000000000,                       // Example amount (1.0 base asset)
		Price:             50000,                            // Example price (50,000 units)
		MarketIndex:       1,                                // Example market index
		ReduceOnly:        false,                            // Example reduce only flag
		PostOnly:          drift.PostOnlyParamMustPostOnly,  // Example post only parameter
		ImmediateOrCancel: false,                            // Example IOC flag
		MaxTs:             &unixNow,                         // Example max timestamp
		TriggerPrice:      proto.Uint64(55000),              // Example trigger price
		TriggerCondition:  drift.OrderTriggerConditionAbove, // Example trigger condition
		OraclePriceOffset: proto.Int32(100),                 // Example oracle price offset
		AuctionDuration:   &auctionDur,                      // Example auction duration
		AuctionStartPrice: proto.Int64(50000),               // Example auction start price
		AuctionEndPrice:   proto.Int64(52000),               // Example auction end price
	}

	// Create the PlaceOrder instruction
	fmt.Println("placing order...")
	placeOrderIx := drift.NewPlaceSpotOrderInstruction(
		orderParams,
		state,
		svmPubkey,
		svmPubkey,
	).Build()
	placeOrderTx, err := solana.NewTransactionBuilder().
		AddInstruction(placeOrderIx).Build()
	if err != nil {
		panic(err)
	}

	svmMsg = svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, placeOrderTx)
	res, err = chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("create order:", res)
}
