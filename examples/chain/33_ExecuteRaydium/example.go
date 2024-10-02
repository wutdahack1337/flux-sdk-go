package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"cosmossdk.io/math"
	astromeshtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/astromesh/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	raydium_cp_swap "github.com/FluxNFTLabs/sdk-go/client/svm/raydium_cp_swap"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	associatedtokenaccount "github.com/gagliardetto/solana-go/programs/associated-token-account"
	"github.com/gagliardetto/solana-go/programs/system"
	"github.com/gagliardetto/solana-go/programs/token"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DenomConfig struct {
	Mint             solana.PublicKey
	DenomUsdtPool    solana.PublicKey
	PoolState        *raydium_cp_swap.PoolState
	TenPowerDecimals uint64
	InitialPriceUsdt uint64
	InitialDeposit   uint64

	DepositAmount uint64
	SwapAmount    uint64
}

const (
	MaxComputeBudget    = 10000000
	InitialBtcUsdtPrice = uint64(70000)
)

var (
	svmDenomMap = map[string]*DenomConfig{
		"btc": {
			TenPowerDecimals: 100_000_000,
			InitialDeposit:   100_000_000,
			InitialPriceUsdt: 70000,

			DepositAmount: 100_000_000,
			SwapAmount:    1_000_000,
		},
		"eth": {
			TenPowerDecimals: 1_000_000_000,
			InitialDeposit:   1_000_000_000,
			InitialPriceUsdt: 3600,

			DepositAmount: 100_000_000,
			SwapAmount:    100_000_000,
		},
		"sol": {
			TenPowerDecimals: 1_000_000_000,
			InitialDeposit:   10_000_000_000,
			InitialPriceUsdt: 200,
			DepositAmount:    1_000_000_000,
			SwapAmount:       50_000_000,
		},
		"usdt": {
			TenPowerDecimals: 1_000_000,
			InitialPriceUsdt: 1,
		},
	}

	raydiumProgramId = solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C")
)

func getPoolNameByPubkey(pk solana.PublicKey) string {
	for denom, config := range svmDenomMap {
		if config.DenomUsdtPool.Equals(pk) {
			return denom + "-usdt"
		}
	}
	return ""
}

func getDenomByMint(mint solana.PublicKey) string {
	for denom, config := range svmDenomMap {
		if config.Mint.Equals(mint) {
			return denom
		}
	}
	return ""
}

func calculateMinimumOutputUsdt(denomAmount uint64, inVaultBalance, outVaultBalance uint64, slippage float64) uint64 {
	price := float64(inVaultBalance) / float64(outVaultBalance) * (1 + slippage)
	return uint64(float64(denomAmount) / price)
}

func calculateLpToken(
	maxToken0Amount, maxToken1Amount uint64,
	token0Vault, token1Vault uint64,
	currentLpAmount uint64,
) (lpAmount, actualToken0Deposit, actualToken1Deposit uint64) {
	// lpAmountToken0Based / currentLpAmount * token0Vault <= token0Amount
	// lpAmountToken1Based / currentLpAmount * token1Vault <= token1Amount
	// => find lpAmount = min(lpAmountToken0Based, lpAmountToken1Based)
	lpAmountToken0Based := math.NewIntFromUint64(maxToken0Amount).Mul(math.NewIntFromUint64(currentLpAmount)).Quo(math.NewIntFromUint64(token0Vault))
	lpAmountToken1Based := math.NewIntFromUint64(maxToken1Amount).Mul(math.NewIntFromUint64(currentLpAmount)).Quo(math.NewIntFromUint64(token1Vault))

	netLpAmount := lpAmountToken0Based
	if netLpAmount.GT(lpAmountToken1Based) {
		netLpAmount = lpAmountToken1Based
	}

	lpAmount = netLpAmount.Uint64()
	actualToken0Deposit = netLpAmount.Mul(math.NewIntFromUint64(token0Vault)).Quo(math.NewIntFromUint64(currentLpAmount)).Uint64()
	actualToken1Deposit = netLpAmount.Mul(math.NewIntFromUint64(token1Vault)).Quo(math.NewIntFromUint64(currentLpAmount)).Uint64()
	return
}

func parseResult(txResp *txtypes.BroadcastTxResponse) (res *astromeshtypes.MsgAstroTransferResponse, err error) {
	hexResp, err := hex.DecodeString(txResp.TxResponse.Data)
	if err != nil {
		panic(err)
	}

	// decode result to get contract address
	var txData sdk.TxMsgData
	if err := txData.Unmarshal(hexResp); err != nil {
		panic(err)
	}

	var r astromeshtypes.MsgAstroTransferResponse
	if err := r.Unmarshal(txData.MsgResponses[0].Value); err != nil {
		return nil, err
	}
	return &r, nil
}

func createSvmAccount(chainClient chainclient.ChainClient, ctx context.Context, senderAddress sdk.Address, svmAccount solana.PublicKey) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	_, err := chainClient.GetSvmAccount(ctx, svmAccount.String())
	msgs := []sdk.Msg{}
	if err != nil {
		if strings.Contains(err.Error(), "Account not existed") {
			fmt.Printf("svm account %s not exist, going to create account\n", svmAccount.String())
			createAccountIx := system.NewCreateAccountInstruction(0, 0, system.ProgramID, senderSvmAccount, svmAccount)
			svmTx, err := solana.NewTransactionBuilder().AddInstruction(createAccountIx.Build()).Build()
			if err != nil {
				// never reach this but added to be safe
				panic(err)
			}
			msgs = append(msgs, svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, svmTx))
		} else {
			panic(err)
		}
	}

	if len(msgs) > 0 {
		res, err := chainClient.SyncBroadcastMsg(msgs...)
		if err != nil {
			panic(err)
		}
		fmt.Println("----- action: Create Svm account ------")
		fmt.Println("cosmos account:", senderAddress.String())
		fmt.Println("svm account:", solana.PublicKey(senderSvmAccount).String())
		fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	}

	fmt.Println("svm account already created:", solana.PublicKey(senderSvmAccount).String())
}

func transferCosmosToSvm(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.Address,
	denom string, amount int64,
) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	res, err := chainClient.SyncBroadcastMsg(&astromeshtypes.MsgAstroTransfer{
		Sender:   senderAddress.String(),
		Receiver: senderAddress.String(),
		SrcPlane: astromeshtypes.Plane_COSMOS,
		DstPlane: astromeshtypes.Plane_SVM,
		Coin:     sdk.NewCoin(denom, math.NewInt(amount)),
	})
	if err != nil {
		panic(err)
	}

	parsedResult, err := parseResult(res)
	if err != nil {
		panic(err)
	}

	ataAccount := svm.MustFindAta(solana.PublicKey(senderSvmAccount), svmtypes.SplToken2022ProgramId, solana.PublicKey(parsedResult.DestinationDenom), svmtypes.AssociatedTokenProgramId)
	fmt.Println("----- action: Transfer ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("cosmos account:", senderAddress.String())
	fmt.Println("svm account:", solana.PublicKey(senderSvmAccount).String())
	fmt.Printf("%s mint svm: %s\n", denom, solana.PublicKey(parsedResult.DestinationDenom).String())
	fmt.Println("ata account (token22):", ataAccount.String())
	fmt.Println("current balance on svm:", mustGetTokenAccount(chainClient, ctx, ataAccount).Amount)
}

func transferBalances(chainClient chainclient.ChainClient, ctx context.Context, senderAddress sdk.Address) {
	transferCosmosToSvm(chainClient, ctx, senderAddress, "btc", 300_000_000)
	transferCosmosToSvm(chainClient, ctx, senderAddress, "sol", 100_000_000_000)
	transferCosmosToSvm(chainClient, ctx, senderAddress, "eth", 3_000_000_000_000_000_000)
	transferCosmosToSvm(chainClient, ctx, senderAddress, "usdt", 200_000_000_000)
}

func createAmmConfig(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumAdmin solana.PublicKey,
	raydiumSwapProgram solana.PublicKey,
) (ammConfigAccount solana.PublicKey) {
	ammConfigAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("amm_config"),
		{0, 0},
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	_, err = chainClient.GetSvmAccount(ctx, ammConfigAccount.String())
	if err == nil {
		fmt.Println("amm config account already created:", ammConfigAccount.String())
		return ammConfigAccount
	}

	createAmmConfigIx := raydium_cp_swap.NewCreateAmmConfigInstruction(
		0,
		1000,         // trade fee (rate) / 10^6 = 0.1%
		2000,         // protocol fee
		1000,         // fund fee
		0,            // create pool fee
		raydiumAdmin, // config owner
		ammConfigAccount,
		solana.SystemProgramID,
	)

	createAmmConfigTx, err := solana.NewTransactionBuilder().AddInstruction(createAmmConfigIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, createAmmConfigTx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: create AMM Config ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("amm config account", ammConfigAccount.String())
	return ammConfigAccount
}

func initializeAmmPool(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumSwapProgram solana.PublicKey,
	raydiumFeeReceiver solana.PublicKey,
	ammConfigAccount solana.PublicKey,
	initialToken0Amount uint64,
	initialToken1Amount uint64,
	token0Mint solana.PublicKey,
	token1Mint solana.PublicKey,
) (poolStateAccount solana.PublicKey, poolState *raydium_cp_swap.PoolState) {
	tokens := []solana.PublicKey{token0Mint, token1Mint}
	initialAmounts := []uint64{initialToken0Amount, initialToken1Amount}
	if bytes.Compare(tokens[0][:], tokens[1][:]) > 0 {
		tokens = []solana.PublicKey{tokens[1], tokens[0]}
		initialAmounts = []uint64{initialAmounts[1], initialAmounts[0]}
	}

	poolStateAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool"),
		ammConfigAccount[:],
		tokens[0][:],
		tokens[1][:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	poolStateResponse, err := chainClient.GetSvmAccount(ctx, poolStateAccount.String())
	if err == nil {
		fmt.Println("pool already created:", poolStateAccount.String())
		poolState := new(raydium_cp_swap.PoolState)
		if err := poolState.UnmarshalWithDecoder(bin.NewBinDecoder(poolStateResponse.Account.Data)); err != nil {
			panic(err)
		}
		return poolStateAccount, poolState
	}

	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	authorityAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("vault_and_lp_mint_auth_seed"),
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	lpMint, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_lp_mint"),
		poolStateAccount[:],
	}, raydiumSwapProgram)

	creatorToken0Ata := svm.MustFindAta(solana.PublicKey(senderSvmAccount), svmtypes.SplToken2022ProgramId, tokens[0], svmtypes.AssociatedTokenProgramId)
	creatorToken1Ata := svm.MustFindAta(solana.PublicKey(senderSvmAccount), svmtypes.SplToken2022ProgramId, tokens[1], svmtypes.AssociatedTokenProgramId)
	creatorLpAta := svm.MustFindAta(solana.PublicKey(senderSvmAccount), solana.TokenProgramID, lpMint, svmtypes.AssociatedTokenProgramId)
	tokens0Vault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_vault"),
		poolStateAccount[:],
		tokens[0][:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	tokens1Vault, _, err := solana.FindProgramAddress([][]byte{
		[]byte("pool_vault"),
		poolStateAccount[:],
		tokens[1][:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	oracleObserver, _, err := solana.FindProgramAddress([][]byte{
		[]byte("observation"),
		poolStateAccount[:],
	}, raydiumSwapProgram)
	if err != nil {
		panic(err)
	}

	createPoolIx := raydium_cp_swap.NewInitializeInstruction(
		initialAmounts[0],
		initialAmounts[1],
		0,
		senderSvmAccount,
		ammConfigAccount,
		authorityAccount,
		poolStateAccount,
		tokens[0],
		tokens[1],
		lpMint,
		creatorToken0Ata,
		creatorToken1Ata,
		creatorLpAta,
		tokens0Vault,
		tokens1Vault,
		raydiumFeeReceiver,
		oracleObserver,
		solana.TokenProgramID,
		svmtypes.SplToken2022ProgramId,
		svmtypes.SplToken2022ProgramId,
		svmtypes.AssociatedTokenProgramId,
		svmtypes.SystemProgramId,
		solana.PublicKey(svmtypes.SysVarRent),
	)

	tx, err := solana.NewTransactionBuilder().AddInstruction(createPoolIx.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: Create pool ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("pool created:", getPoolNameByPubkey(poolStateAccount))
	fmt.Println("pool state account:", poolStateAccount.String())
	fmt.Println("creator owner lp amount:", mustGetTokenAccount(chainClient, ctx, creatorLpAta).Amount)

	poolStateResponse, err = chainClient.GetSvmAccount(ctx, poolStateAccount.String())
	if err != nil {
		panic(err)
	}
	poolState = new(raydium_cp_swap.PoolState)
	if err := poolState.UnmarshalWithDecoder(bin.NewBinDecoder(poolStateResponse.Account.Data)); err != nil {
		panic(err)
	}
	return poolStateAccount, poolState
}

func mustGetTokenAccount(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	account solana.PublicKey,
) *token.Account {
	var tokenAcc token.Account
	acc, err := chainClient.GetSvmAccount(ctx, account.String())
	if err != nil {
		panic(err)
	}

	if err := tokenAcc.UnmarshalWithDecoder(bin.NewBinDecoder(acc.Account.Data)); err != nil {
		panic(err)
	}
	return &tokenAcc
}

func swapBaseInput(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	raydiumSwapProgram solana.PublicKey,
	authorityAccount solana.PublicKey, // TODO: What is this for?
	amountIn uint64,
	minAmountOut uint64,
	inputTokenAccount solana.PublicKey,
	outputTokenAccount solana.PublicKey,
	poolStateAccount solana.PublicKey,
	poolState *raydium_cp_swap.PoolState,
	inputVault solana.PublicKey,
	outputVault solana.PublicKey,
	inputTokenMint solana.PublicKey,
	outputTokenMint solana.PublicKey,
	observerState solana.PublicKey,
) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ix := raydium_cp_swap.NewSwapBaseInputInstruction(
		amountIn,
		uint64(minAmountOut),
		senderSvmAccount,
		authorityAccount,
		poolState.AmmConfig,
		poolStateAccount,
		inputTokenAccount,
		outputTokenAccount,
		inputVault,
		outputVault,
		svmtypes.SplToken2022ProgramId,
		svmtypes.SplToken2022ProgramId,
		inputTokenMint,
		outputTokenMint,
		observerState,
	)

	tx, err := solana.NewTransactionBuilder().AddInstruction(ix.Build()).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: Swap ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
}

func createNativeMint(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
) {
	_, err := chainClient.GetSvmAccount(ctx, svm.Sol22NativeMint.String())
	if err == nil {
		fmt.Println("sol native mint already created:", svm.Sol22NativeMint.String())
		return
	}

	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	createMintIx := svm.NewCreateNativeMintInstruction(
		senderSvmAccount, svm.Sol22NativeMint, solana.SystemProgramID,
	)
	tx, err := solana.NewTransactionBuilder().AddInstruction(createMintIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, 1000000, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: create native mint ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("sol native mint created:", svm.Sol22NativeMint.String())
}

func createFeeReceiverAccount(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	owner solana.PublicKey,
) solana.PublicKey {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ownerSolAta := svm.MustFindAta(owner, svmtypes.SplToken2022ProgramId, svm.Sol22NativeMint, svmtypes.AssociatedTokenProgramId)
	accData, err := chainClient.GetSvmAccount(ctx, ownerSolAta.String())
	if err == nil {
		fmt.Println("sol receiver ATA created:", ownerSolAta.String())
		var a = new(token.Account)
		err := a.UnmarshalWithDecoder(bin.NewBinDecoder(accData.Account.Data))
		if err != nil {
			panic(err)
		}
		return ownerSolAta
	}

	createAtaIx := associatedtokenaccount.NewCreateInstruction(
		senderSvmAccount,
		owner,
		svm.Sol22NativeMint,
	).Build()

	// native anchor-go doesn't support token2022 so the value is wrong, we can just update these
	createAtaIx.Accounts()[1].PublicKey = ownerSolAta
	createAtaIx.Accounts()[5].PublicKey = svmtypes.SplToken2022ProgramId

	tx, err := solana.NewTransactionBuilder().AddInstruction(createAtaIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: create fee receiver ATA ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("admin SOL (token 2022) ata:", ownerSolAta)
	return ownerSolAta
}

func deposit(
	chainClient chainclient.ChainClient,
	ctx context.Context,
	senderAddress sdk.AccAddress,
	maxToken0Amount uint64,
	maxToken1Amount uint64,
	token0Account solana.PublicKey,
	token1Account solana.PublicKey,
	owner solana.PublicKey,
	authority solana.PublicKey, // program PDA as mint authority
	poolStateAccount solana.PublicKey,
	poolState *raydium_cp_swap.PoolState,
) {
	senderSvmAccount := solana.PublicKey(ethcrypto.Keccak256(senderAddress.Bytes()))
	ownerLpToken := svm.MustFindAta(
		senderSvmAccount, solana.TokenProgramID, poolState.LpMint, svmtypes.AssociatedTokenProgramId,
	)

	// get token 0 vault balance
	token0VaultBalance := mustGetTokenAccount(chainClient, ctx, poolState.Token0Vault)
	// get token 1 vault balance
	token1VaultBalance := mustGetTokenAccount(chainClient, ctx, poolState.Token1Vault)

	lpAmount, _, _ := calculateLpToken(
		maxToken0Amount, maxToken1Amount,
		token0VaultBalance.Amount-(poolState.ProtocolFeesToken0+poolState.FundFeesToken0),
		token1VaultBalance.Amount-(poolState.ProtocolFeesToken1+poolState.FundFeesToken1),
		poolState.LpSupply,
	)
	depositIx := raydium_cp_swap.NewDepositInstruction(
		lpAmount,
		maxToken0Amount,
		maxToken1Amount,
		senderSvmAccount,
		authority,
		poolStateAccount,
		ownerLpToken,
		token0Account,
		token1Account,
		poolState.Token0Vault,
		poolState.Token1Vault,
		solana.TokenProgramID,
		svmtypes.SplToken2022ProgramId,
		poolState.Token0Mint,
		poolState.Token1Mint,
		poolState.LpMint,
	).Build()

	tx, err := solana.NewTransactionBuilder().AddInstruction(depositIx).Build()
	if err != nil {
		panic(err)
	}

	res, err := chainClient.SyncBroadcastMsg(svmtypes.ToCosmosMsg([]string{senderAddress.String()}, MaxComputeBudget, tx))
	if err != nil {
		panic(err)
	}

	fmt.Println("----- action: Deposit ------")
	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, "/", res.TxResponse.GasWanted)
	fmt.Println("deposited to vault:", getPoolNameByPubkey(poolStateAccount))
	fmt.Printf("token 0 (%s) vault amount after deposit: %d\n", getDenomByMint(poolState.Token0Mint), mustGetTokenAccount(chainClient, ctx, poolState.Token0Vault).Amount)
	fmt.Printf("token 1 (%s) vault amount after deposit: %d\n", getDenomByMint(poolState.Token1Mint), mustGetTokenAccount(chainClient, ctx, poolState.Token1Vault).Amount)
	fmt.Println("owner's lp new amount:", mustGetTokenAccount(chainClient, ctx, ownerLpToken).Amount)
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

	ctx := context.Background()
	senderSvmAddress := solana.PublicKey(ethcrypto.Keccak256(senderAddress))
	createSvmAccount(chainClient, ctx, senderAddress, senderSvmAddress)

	// get btc, usdt denom on svm, if doesn't exist then transfer some
	transferBalances(chainClient, ctx, senderAddress)

	// get mints
	for denom := range svmDenomMap {
		btcLink, err := chainClient.GetDenomLink(ctx, astromeshtypes.Plane_COSMOS, denom, astromeshtypes.Plane_SVM)
		if err != nil {
			panic(err)
		}
		denomBytes, _ := hex.DecodeString(btcLink.DstAddr)
		mint := solana.PublicKey(denomBytes)
		svmDenomMap[denom].Mint = mint
	}

	// create fee receiver account
	adminAccount := solana.MustPublicKeyFromBase58("GThUX1Atko4tqhN2NaiTazWSeFWMuiUvfFnyJyUghFMJ")
	createNativeMint(chainClient, ctx, senderAddress)
	feeReceiverAccount := createFeeReceiverAccount(chainClient, ctx, senderAddress, adminAccount)

	// create amm config
	raydium_cp_swap.SetProgramID(raydiumProgramId)
	ammConfigAccount := createAmmConfig(chainClient, ctx, senderAddress, adminAccount, raydiumProgramId)

	// create pool
	authorityAccount, _, err := solana.FindProgramAddress([][]byte{
		[]byte("vault_and_lp_mint_auth_seed"),
	}, raydiumProgramId)
	if err != nil {
		panic(err)
	}

	// initialize amm pools
	for denom, config := range svmDenomMap {
		if denom == "usdt" {
			continue
		}

		denomAmount := config.InitialDeposit
		tenPowerDecimals := config.TenPowerDecimals
		usdtAmount := denomAmount * config.InitialPriceUsdt / tenPowerDecimals * 1000_000
		poolAccount, poolState := initializeAmmPool(
			chainClient,
			ctx, senderAddress,
			raydiumProgramId,
			feeReceiverAccount,
			ammConfigAccount,
			denomAmount, usdtAmount,
			config.Mint,
			svmDenomMap["usdt"].Mint,
		)

		svmDenomMap[denom].DenomUsdtPool = poolAccount
		svmDenomMap[denom].PoolState = poolState
	}

	// deposit (provide liquidity) for each pool
	for denom, config := range svmDenomMap {
		if denom == "usdt" {
			continue
		}

		traderDenomAta := svm.MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, config.Mint, svmtypes.AssociatedTokenProgramId)
		traderUsdtAta := svm.MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, svmDenomMap["usdt"].Mint, svmtypes.AssociatedTokenProgramId)
		denomDepositAmount := uint64(config.DepositAmount)
		decimalDivisor := config.TenPowerDecimals

		usdtDepositAmount := denomDepositAmount * config.InitialPriceUsdt / decimalDivisor * 1000_000
		amounts := []uint64{denomDepositAmount, usdtDepositAmount}
		atas := []solana.PublicKey{traderDenomAta, traderUsdtAta}
		if bytes.Compare(config.Mint[:], svmDenomMap["usdt"].Mint[:]) > 0 {
			amounts = []uint64{amounts[1], amounts[0]}
			atas = []solana.PublicKey{atas[1], atas[0]}
		}

		deposit(
			chainClient,
			ctx,
			senderAddress,
			amounts[0],
			amounts[1],
			atas[0],
			atas[1],
			senderSvmAddress,
			authorityAccount,
			config.DenomUsdtPool,
			config.PoolState,
		)
	}

	// swap
	for denom, config := range svmDenomMap {
		if denom == "usdt" {
			continue
		}

		denomAta := svm.MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, config.Mint, svmtypes.AssociatedTokenProgramId)
		usdtAta := svm.MustFindAta(senderSvmAddress, svmtypes.SplToken2022ProgramId, svmDenomMap["usdt"].Mint, svmtypes.AssociatedTokenProgramId)

		denomAmount := mustGetTokenAccount(chainClient, ctx, denomAta).Amount
		usdtAmount := mustGetTokenAccount(chainClient, ctx, usdtAta).Amount
		swapAmount := uint64(config.SwapAmount)
		decimalDivisor := config.TenPowerDecimals
		// swap denom > usdt, we need to sort to ensure we use correct in => out order
		inVault, outVault := config.PoolState.Token0Vault, config.PoolState.Token1Vault
		if !config.PoolState.Token0Mint.Equals(config.Mint) {
			inVault, outVault = outVault, inVault
		}
		inVaultBalance := mustGetTokenAccount(chainClient, ctx, inVault).Amount
		outVaultBalance := mustGetTokenAccount(chainClient, ctx, outVault).Amount

		swapBaseInput(
			chainClient,
			ctx,
			senderAddress,
			raydiumProgramId,
			authorityAccount,
			swapAmount,
			calculateMinimumOutputUsdt(swapAmount, inVaultBalance, outVaultBalance, 0.1), // 10% slippage
			denomAta,
			usdtAta,
			config.DenomUsdtPool,
			config.PoolState,
			inVault,
			outVault,
			config.Mint,
			svmDenomMap["usdt"].Mint,
			config.PoolState.ObservationKey,
		)

		denomAmountAfter := mustGetTokenAccount(chainClient, ctx, denomAta).Amount
		usdtAmountAfter := mustGetTokenAccount(chainClient, ctx, usdtAta).Amount
		denomChange := denomAmount - denomAmountAfter
		usdtChange := usdtAmountAfter - usdtAmount
		// convert to human readable format by dividing to their decimals
		fmt.Println("sold", float64(denomChange)/float64(decimalDivisor), strings.ToUpper(denom), "for", float64(usdtChange)/float64(1000000), "USDT")
	}
}
