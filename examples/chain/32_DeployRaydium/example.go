package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	svmtypes "github.com/FluxNFTLabs/sdk-go/chain/modules/svm/types"
	chaintypes "github.com/FluxNFTLabs/sdk-go/chain/types"
	chainclient "github.com/FluxNFTLabs/sdk-go/client/chain"
	"github.com/FluxNFTLabs/sdk-go/client/common"
	"github.com/FluxNFTLabs/sdk-go/client/svm"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/system"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const MaxComputeBudget = 10000000

func BuildInitAccountsMsg(
	senderAddr sdk.AccAddress,
	programSize int,
	programPubkey solana.PublicKey,
	programBufferPubkey solana.PublicKey,
) *types.MsgTransaction {
	callerPubkey := solana.PublicKeyFromBytes(ethcrypto.Keccak256(senderAddr))
	initTxBuilder := solana.NewTransactionBuilder()

	createAccountIx := system.NewCreateAccountInstruction(
		0, uint64(programSize)+48,
		solana.BPFLoaderUpgradeableProgramID, callerPubkey,
		programBufferPubkey,
	).Build()
	createProgramAccountIx := system.NewCreateAccountInstruction(
		0, 36,
		solana.BPFLoaderUpgradeableProgramID,
		callerPubkey,
		programPubkey,
	).Build()
	initBufferAccountIx := solana.NewInstruction(
		solana.BPFLoaderUpgradeableProgramID,
		solana.AccountMetaSlice{
			&solana.AccountMeta{
				PublicKey:  programBufferPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
			&solana.AccountMeta{
				PublicKey:  callerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
		},
		[]byte{0, 0, 0, 0},
	)

	initTxBuilder.
		AddInstruction(createAccountIx).
		AddInstruction(createProgramAccountIx).
		AddInstruction(initBufferAccountIx)

	initTx, err := initTxBuilder.Build()
	if err != nil {
		panic(initTx)
	}

	return svmtypes.ToCosmosMsg([]string{senderAddr.String()}, MaxComputeBudget, initTx)
}

func BuildDeployMsg(
	senderAddr sdk.AccAddress,
	programPubkey solana.PublicKey,
	programBufferPubkey solana.PublicKey,
	programBz []byte,
) *types.MsgTransaction {
	callerPubkey := solana.PublicKeyFromBytes(ethcrypto.Keccak256(senderAddr))
	windowSize := 1200 // chunk by chunk, 1223
	programSize := len(programBz)
	txBuilder := solana.NewTransactionBuilder()
	for idx := 0; idx < programSize; {
		end := idx + windowSize
		if end > programSize {
			end = programSize
		}

		codeSlice := programBz[idx:end]
		data := svm.MustMarshalIxData(svm.WriteBuffer{
			Offset: uint32(idx),
			Data:   codeSlice,
		})

		writeBufferIx := solana.NewInstruction(
			solana.BPFLoaderUpgradeableProgramID,
			solana.AccountMetaSlice{
				&solana.AccountMeta{
					PublicKey:  programBufferPubkey,
					IsWritable: true,
					IsSigner:   false,
				},
				&solana.AccountMeta{
					PublicKey:  callerPubkey,
					IsWritable: true,
					IsSigner:   true,
				},
			},
			data,
		)
		txBuilder = txBuilder.AddInstruction(writeBufferIx)
		idx = end
	}

	data := svm.MustMarshalIxData(svm.DeployWithMaxDataLen{
		DataLen: uint64(len(programBz)) + 48,
	})

	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		panic(err)
	}

	deployIx := solana.NewInstruction(
		solana.BPFLoaderUpgradeableProgramID,
		solana.AccountMetaSlice{
			&solana.AccountMeta{
				PublicKey:  callerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
			&solana.AccountMeta{
				PublicKey:  programExecutablePubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  programPubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  programBufferPubkey,
				IsWritable: true,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SysVarRentPubkey,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SysVarClockPubkey,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  solana.SystemProgramID,
				IsWritable: false,
				IsSigner:   false,
			},
			&solana.AccountMeta{
				PublicKey:  callerPubkey,
				IsWritable: true,
				IsSigner:   true,
			},
		},
		data,
	)

	tx, err := txBuilder.AddInstruction(deployIx).Build()
	if err != nil {
		panic(err)
	}
	return svmtypes.ToCosmosMsg([]string{senderAddr.String()}, MaxComputeBudget, tx)
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

	programBz, err := os.ReadFile("examples/chain/30_DeployRaydium/raydium_cp_swap.so")
	if err != nil {
		panic(err)
	}

	programPubkey := solana.MustPublicKeyFromBase58("CPMMoo8L3F4NbTegBCKVNunggL7H1ZpdTHKxQB5qKP1C") // TODO: Replace expected pubkey here
	programBufferPubkey := solana.NewWallet().PublicKey()
	initAccountMsg := BuildInitAccountsMsg(senderAddress, len(programBz), programPubkey, programBufferPubkey)
	deployMsg := BuildDeployMsg(senderAddress, programPubkey, programBufferPubkey, programBz)
	fmt.Println("number of instruction to deploy:", len(deployMsg.Instructions))
	//AsyncBroadcastMsg, SyncBroadcastMsg, QueueBroadcastMsg
	res, err := chainClient.SyncBroadcastMsg(
		initAccountMsg, deployMsg,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println("tx hash:", res.TxResponse.TxHash)
	fmt.Println("tx code:", res.TxResponse.Code)
	fmt.Println("gas used/want:", res.TxResponse.GasUsed, res.TxResponse.GasWanted)
	fmt.Println("program pubkey:", programPubkey.String())

	programExecutablePubkey, _, err := solana.FindProgramAddress([][]byte{programPubkey[:]}, solana.BPFLoaderUpgradeableProgramID)
	if err != nil {
		panic(err)
	}
	fmt.Println("program executable data pubkey:", programExecutablePubkey.String())
}
