package main

import (
	"context"
	"errors"
	"fmt"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
	"github.com/goccy/go-json"
	ffmpeg "github.com/u2takey/ffmpeg-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"math"
	"os"
	"strconv"
)

func main() {
	cc, err := grpc.Dial("localhost:4444", grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer cc.Close()
	if err != nil {
		panic(err)
	}
	client := types.NewAPIClient(cc)

	// init upload client
	uc, err := client.Upload(context.Background())
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
	metadata := &types.Metadata{
		Path:       "series_0_1_product" + extension,
		Encrypted:  false,
		Type:       types.ContentType_Audio,
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

	// upload metadata
	err = uc.Send(&types.StreamMsg{
		Content: &types.StreamMsg_Metadata{
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
		err = uc.Send(&types.StreamMsg{
			Content: &types.StreamMsg_MediaChunk{MediaChunk: chunk},
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
