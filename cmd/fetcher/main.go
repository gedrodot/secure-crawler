package main

import (
	"context"
	"io"
	"net"
	"net/http"
	pb "securecrawler/protos"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	_portNumber = "[::]:10116"
)

type GrpcFetcherServer struct {
	pb.UnimplementedFetcherServiceServer
}

func NewFetcherServer() (*GrpcFetcherServer, error) {
	return &GrpcFetcherServer{}, nil
}

// Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
func (s *GrpcFetcherServer) Fetch(ctx context.Context, req *pb.FetchRequest) (*pb.FetchResponse, error) {
	resp, err := http.Get(req.GetUrl())
	if err != nil {
		return &pb.FetchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &pb.FetchResponse{}, err
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return &pb.FetchResponse{}, err
	}

	doc, err := io.ReadAll(resp.Body)
	if err != nil {
		return &pb.FetchResponse{}, err
	}

	return &pb.FetchResponse{Body: string(doc)}, nil
}

func main() {
	creds := insecure.NewCredentials()
	service, _ := NewFetcherServer()

	server := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterFetcherServiceServer(server, service)
	reflection.Register(server)

	listener, err := net.Listen("tcp", _portNumber)
	if err != nil {
		return
	}

	if err := server.Serve(listener); err != nil {
		return
	}

}
