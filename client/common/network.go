package common

import (
	"context"
	"net"
	"path"
	"runtime"
	"strings"

	"google.golang.org/grpc/credentials"
)

type Network struct {
	LcdEndpoint       string
	TmEndpoint        string
	ChainGrpcEndpoint string
	ChainTlsCert      credentials.TransportCredentials
	ChainId           string
	Name              string
}

func getFileAbsPath(relativePath string) string {
	_, filename, _, _ := runtime.Caller(0)
	return path.Join(path.Dir(filename), relativePath)
}

func LoadNetwork(name string, node string) Network {
	switch name {
	case "local":
		return Network{
			LcdEndpoint:       "http://localhost:10337",
			TmEndpoint:        "http://localhost:26657",
			ChainGrpcEndpoint: "localhost:9900",
			ChainId:           "flux-1",
			Name:              "local",
		}

	case "devnet":
		return Network{
			LcdEndpoint:       "https://devnet.lcd.fluxnft.space/",
			TmEndpoint:        "https://devnet.tm.fluxnft.space/",
			ChainGrpcEndpoint: "52.22.4.129:9900",
			ChainId:           "flux-1",
			Name:              "devnet",
		}
	}

	return Network{}
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func DialerFunc(ctx context.Context, addr string) (net.Conn, error) {
	return Connect(addr)
}

// Connect dials the given address and returns a net.Conn. The protoAddr argument should be prefixed with the protocol,
// eg. "tcp://127.0.0.1:8080" or "unix:///tmp/test.sock"
func Connect(protoAddr string) (net.Conn, error) {
	proto, address := ProtocolAndAddress(protoAddr)
	conn, err := net.Dial(proto, address)
	return conn, err
}

// ProtocolAndAddress splits an address into the protocol and address components.
// For instance, "tcp://127.0.0.1:8080" will be split into "tcp" and "127.0.0.1:8080".
// If the address has no protocol prefix, the default is "tcp".
func ProtocolAndAddress(listenAddr string) (string, string) {
	protocol, address := "tcp", listenAddr
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	}
	return protocol, address
}
