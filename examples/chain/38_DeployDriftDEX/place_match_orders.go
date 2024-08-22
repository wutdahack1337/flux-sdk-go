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

	driftProgramId solana.PublicKey

	oracleBtc = solana.MustPublicKeyFromBase58("3HRnxmtHQrHkooPdFZn5ZQbPTKGvBSyoTi4VVkkoT6u6")

	usdtMint solana.PublicKey
	btcMint  solana.PublicKey
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

func deposit(
	chainClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	depositAmount uint64,
	mint solana.PublicKey,
	marketIndex uint16,
) {
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, driftProgramId)

	spotMarketVault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market_vault"),
		uint16ToLeBytes(0),
	}, driftProgramId)
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
		[]byte("user"), svmPubkey[:], []byte{0, 0},
	}, driftProgramId)

	userStats, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user_stats"), svmPubkey[:],
	}, driftProgramId)

	spotMarket, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(marketIndex),
	}, driftProgramId)
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
		0, uint64(depositAmount), false, state, user, userStats, svmPubkey, spotMarketVault, userTokenAccount, svmtypes.SplToken2022ProgramId,
	)

	depositIxBuilder.Append(&solana.AccountMeta{
		PublicKey:  spotMarket,
		IsWritable: true,
		IsSigner:   false,
	})
	depositIx := depositIxBuilder.Build()

	depositTx, err := solana.NewTransactionBuilder().
		AddInstruction(initializeUserStatsIx).
		AddInstruction(initializeUserIx).
		AddInstruction(depositIx).Build()
	if err != nil {
		panic(err)
	}

	svmMsg := svm.ToCosmosMsg([]string{chainClient.FromAddress().String()}, 1_000_000, depositTx)
	res, err := chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("=== deposit ===")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func placeOrder(
	chainClient chainclient.ChainClient,
	svmPubkey solana.PublicKey,
	price uint64,
	baseAssetAmount uint64,
	direction drift.PositionDirection,
) {
	senderAddress := chainClient.FromAddress()
	state, _, err := solana.FindProgramAddress([][]byte{
		[]byte("drift_state"),
	}, driftProgramId)

	user, _, err := solana.FindProgramAddress([][]byte{
		[]byte("user"), svmPubkey[:], []byte{0, 0},
	}, driftProgramId)

	// create market
	// Generate PDA for spot_market
	spotMarketUsdt, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(0),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// // Generate PDA for spot_market_vault
	// spotMarketUsdtVault, _, err := solana.FindProgramAddress([][]byte{
	// 	[]byte("spot_market_vault"),
	// 	uint16ToLeBytes(0),
	// }, driftProgramId)
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate PDA for insurance_fund_vault
	// insuranceFundUsdtVault, _, err := solana.FindProgramAddress([][]byte{
	// 	[]byte("insurance_fund_vault"),
	// 	uint16ToLeBytes(0),
	// }, driftProgramId)
	// if err != nil {
	// 	panic(err)
	// }

	spotMarketBtc, _, err := solana.FindProgramAddress([][]byte{
		[]byte("spot_market"),
		uint16ToLeBytes(1),
	}, driftProgramId)
	if err != nil {
		panic(err)
	}

	// // // Generate PDA for spot_market_vault
	// spotMarketBtcVault, _, err := solana.FindProgramAddress([][]byte{
	// 	[]byte("spot_market_vault"),
	// 	uint16ToLeBytes(1),
	// }, driftProgramId)
	// if err != nil {
	// 	panic(err)
	// }

	// // Generate PDA for insurance_fund_vault
	// insuranceFundBtcVault, _, err := solana.FindProgramAddress([][]byte{
	// 	[]byte("insurance_fund_vault"),
	// 	uint16ToLeBytes(1),
	// }, driftProgramId)
	// if err != nil {
	// 	panic(err)
	// }

	// oracleUsdt := svmtypes.SystemProgramId // default pubkey
	// Define other necessary public keys
	// admin := svmPubkey
	// rent := solana.SysVarRentPubkey
	// systemProgram := solana.SystemProgramID
	// tokenProgram := svmtypes.SplToken2022ProgramId

	// optimalUtilization := uint32(8000)
	// optimalBorrowRate := uint32(500)
	// maxBorrowRate := uint32(1000)
	// oracleSourceQuote := drift.OracleSourceQuoteAsset
	// oracleSourcePyth := drift.OracleSourcePyth // TODO: Use pyth later
	// place spot market order
	// Define the OrderParams with default or specified values
	unixNow := time.Now().Unix()
	auctionDur := uint8(100)
	orderParams := drift.OrderParams{
		OrderType:         drift.OrderTypeLimit,
		MarketType:        drift.MarketTypeSpot,
		Direction:         drift.PositionDirectionLong,
		UserOrderId:       1,
		BaseAssetAmount:   baseAssetAmount,
		Price:             price,
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

	placeOrderTx, err := solana.NewTransactionBuilder().AddInstruction(placeOrderIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	fmt.Println("== place order ==")
	svmMsg := svm.ToCosmosMsg([]string{senderAddress.String()}, 1000_000, placeOrderTx)
	res, err := chainClient.SyncBroadcastMsg(svmMsg)
	if err != nil {
		panic(err)
	}
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
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

	clientCtx2, partnerAddress, err := chaintypes.NewClientContext(
		network.ChainId,
		"signer4",
		kr,
	)
	if err != nil {
		panic(err)
	}
	clientCtx2 = clientCtx2.WithGRPCClient(cc)

	// init chain client
	chainClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	partnerClient, err := chainclient.NewChainClient(
		clientCtx,
		common.OptionGasPrices("500000000lux"),
	)
	if err != nil {
		panic(err)
	}

	// check and link accounts
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

	isSvmLinked, partnerSvmPubkey, err := chainClient.GetSVMAccountLink(context.Background(), partnerAddress)
	if err != nil {
		panic(err)
	}
	if !isSvmLinked {
		svmKey := ed25519.GenPrivKey() // Good practice: Backup this private key
		res, err := partnerClient.LinkSVMAccount(svmKey)
		if err != nil {
			panic(err)
		}
		fmt.Println("linked", partnerAddress, "to svm address:", base58.Encode(svmKey.PubKey().Bytes()), "txHash:", res.TxResponse.TxHash)
		partnerSvmPubkey = solana.PublicKey(svmKey.PubKey().Bytes())
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
	driftProgramId = solana.PublicKeyFromBytes(programSvmPrivKey.PubKey().Bytes())
	drift.SetProgramID(driftProgramId)

	fmt.Println("drift programId:", driftProgramId.String())
	usdtMintHex := "1c46743a65e0fe89a65a9fe498d8cfa813480358fc1dd4658c6cf842d0560c92"
	usdtMintBz, _ := hex.DecodeString(usdtMintHex)
	usdtMint = solana.PublicKeyFromBytes(usdtMintBz)

	btcMintHex := "0811ed5c81d01548aa6cb5177bdeccc835465be58d4fa6b26574f5f7fd258bcd"
	btcMintBz, _ := hex.DecodeString(btcMintHex)
	btcMint = solana.PublicKeyFromBytes(btcMintBz)
	deposit(
		chainClient,
		svmPubkey,
		650_000_000,
		usdtMint, 0,
	)

	deposit(
		partnerClient,
		partnerSvmPubkey,
		10_000_000,
		btcMint, 1,
	)

	placeOrder(
		chainClient,
		svmPubkey,
		65000_000_000,
		10_000_000,
		drift.PositionDirectionLong,
	)

	placeOrder(
		chainClient,
		svmPubkey,
		65000_000_000,
		10_000_000,
		drift.PositionDirectionShort,
	)
}
