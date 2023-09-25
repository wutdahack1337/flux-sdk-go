package main

import (
	"context"
	"encoding/json"
	"errors"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
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

	//init upload client
	uc, err := client.Upload(context.Background())
	if err != nil {
		panic(err)
	}

	// read video chunk info
	extension := ".mov"
	path := "examples/indexer/2_UploadStream/samples/beach" + extension
	chunkCount, chunkTime, err := getVideoChunkInfo(path, 30, 3)
	if err != nil {
		panic(err)
	}
	metadata := &types.Metadata{
		Path:       "series_0_0_product" + extension,
		Encrypted:  false,
		Type:       types.ContentType_Video,
		ChunkCount: chunkCount,
		ChunkTime:  chunkTime,
		Thumbnail:  "",
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

type VideoInfo struct {
	CodecType string `json:"codec_type"`
	Width     uint   `json:"width"`
	Height    uint   `json:"height"`
	BitRate   string `json:"bit_rate"`
	Duration  string `json:"duration"`
}

type VideoMetadata struct {
	Streams []VideoInfo `json:"streams"`
}

func getVideoChunkInfo(fileName string, maxChunkSize float64, minChunkDuration float64) (uint64, uint64, error) {
	data, err := ffmpeg.Probe(fileName)
	if err != nil {
		return 0, 0, err
	}

	videoMetadata := &VideoMetadata{}
	err = json.Unmarshal([]byte(data), videoMetadata)
	if err != nil {
		return 0, 0, err
	}

	for _, info := range videoMetadata.Streams {
		if info.CodecType == "video" {
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
				return 0, 0, errors.New("video bit rate is too high")
			}

			return uint64(chunkCount), uint64(chunkDuration), nil
		}
	}

	return 0, 0, errors.New("video metadata not found")
}
