package main

import (
	"context"
	types "github.com/FluxNFTLabs/sdk-go/chain/indexer/media"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
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

	// read file in chunks
	extension := ".jpg"
	path := "examples/indexer/2_UploadStream/samples/bird" + extension
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
			Metadata: &types.Metadata{
				Path:      "series_0_2_product" + extension,
				Encrypted: false,
				Type:      types.ContentType_Static,
			},
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
