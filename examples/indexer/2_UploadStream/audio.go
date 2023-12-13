package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/FluxNFTLabs/sdk-go/chain/indexer/account"
	"github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ethsecp256k1"
	"github.com/cosmos/cosmos-sdk/types/bech32"
	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	grpcmetadata "google.golang.org/grpc/metadata"
	"math"
	"os"
	"strconv"
)

func main() {
	senderPrivKey := ethsecp256k1.PrivKey{Key: common.Hex2Bytes("88CBEAD91AEE890D27BF06E003ADE3D4E952427E88F88D31D61D3EF5E5D54305")}
	senderAddr, _ := bech32.ConvertAndEncode("lux", senderPrivKey.PubKey().Address().Bytes())

	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	mediaClient := media.NewAPIClient(cc)

	cc, err = grpc.Dial("localhost:4454", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	accountClient := account.NewAPIClient(cc)

	ctx := context.Background()
	acc, err := accountClient.GetAccount(ctx, &account.GetAccountRequest{Address: senderAddr})
	if err != nil {
		panic(err)
	}

	// read audio chunk info
	extension := ".mp3"
	path := "examples/indexer/2_UploadStream/samples/hello" + extension
	chunkCount, chunkTime, err := getAudioInfo(path, 2, 125)
	if err != nil {
		panic(err)
	}
	fmt.Println(chunkCount, chunkTime)
	metadata := &media.Metadata{
		Path:       "series_0_1_product" + extension,
		Encrypted:  false,
		Type:       media.ContentType_Audio,
		ChunkCount: chunkCount,
		ChunkTime:  chunkTime,
	}

	// read file in chunks
	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// compute file size
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	fileSize := info.Size()
	fileIdx := int64(0)

	// prepare header & ctx
	req := media.StreamMsg{
		Content: &media.StreamMsg_Metadata{
			Metadata: metadata,
		},
	}
	nonce := []byte(strconv.FormatUint(acc.Nonce, 10))
	reqBz, _ := req.Marshal()
	reqHash := ethcrypto.Keccak256(append(reqBz, nonce...))
	senderEthPk, _ := ethcrypto.ToECDSA(senderPrivKey.Bytes())
	reqSig, err := ethcrypto.Sign(reqHash, senderEthPk)
	if err != nil {
		panic(err)
	}

	ctx = grpcmetadata.NewOutgoingContext(ctx, grpcmetadata.MD{
		"sender":    []string{senderAddr},
		"signature": []string{common.Bytes2Hex(reqSig)},
	})

	// init upload client
	uc, err := mediaClient.Upload(ctx)
	if err != nil {
		panic(err)
	}

	// upload metadata
	err = uc.Send(&media.StreamMsg{
		Content: &media.StreamMsg_Metadata{
			Metadata: metadata,
		},
	})
	if err != nil {
		panic(err)
	}

	// upload chunks
	for {
		// compute remain bytes
		remainBytes := fileSize - fileIdx
		if remainBytes == 0 {
			break
		}

		// decide chunk size to read
		chunkSize := int64(1000000)
		if chunkSize > remainBytes {
			chunkSize = remainBytes
		}

		// get chunk
		chunk := make([]byte, chunkSize)
		idx, err := file.Read(chunk)
		if err != nil {
			panic(err)
		}
		fileIdx += int64(idx)

		// upload chunk
		err = uc.Send(&media.StreamMsg{
			Content: &media.StreamMsg_MediaChunk{MediaChunk: chunk},
		})
	}

	// close stream
	if _, err = uc.CloseAndRecv(); err != nil {
		panic(err)
	}
}

type AudioInfo struct {
	CodecType string `json:"codec_type"`
	Width     uint   `json:"width"`
	Height    uint   `json:"height"`
	BitRate   string `json:"bit_rate"`
	Duration  string `json:"duration"`
}

type AudioMetadata struct {
	Streams []AudioInfo `json:"streams"`
}

func getAudioInfo(fileName string, maxChunkSize float64, minChunkDuration float64) (uint64, uint64, error) {
	data, err := ffmpeg.Probe(fileName)
	if err != nil {
		return 0, 0, err
	}

	audioMetadata := &AudioMetadata{}
	err = json.Unmarshal([]byte(data), audioMetadata)
	if err != nil {
		return 0, 0, err
	}

	for _, info := range audioMetadata.Streams {
		if info.CodecType == "audio" {
			// compute bit rate and chunk size
			bitRate, err := strconv.ParseFloat(info.BitRate, 64)
			if err != nil {
				return 0, 0, err
			}
			duration, err := strconv.ParseFloat(info.Duration, 64)
			if err != nil {
				return 0, 0, err
			}

			// compute reasonable chunk size according to maximum bit rate
			mbRate := bitRate / 8 / 1000 / 1000
			chunkCount := math.Ceil(mbRate * duration / maxChunkSize)
			chunkDuration := math.Ceil(duration / chunkCount)

			// this means each chunk must fit at least x seconds
			if mbRate*minChunkDuration > maxChunkSize {
				return 0, 0, errors.New("audio bit rate is too high")
			}

			return uint64(chunkCount), uint64(chunkDuration), nil
		}
	}

	return 0, 0, errors.New("audio metadata not found")
}
