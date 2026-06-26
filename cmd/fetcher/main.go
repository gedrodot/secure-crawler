package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	pb "securecrawler/protos"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	_portNumber = "[::]:10116"
)

type GrpcFetcherServer struct {
	pb.UnimplementedFetcherServiceServer
	httpClient *http.Client
}

func NewFetcherServer() (*GrpcFetcherServer, error) {
	return &GrpcFetcherServer{
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

// Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
func (s *GrpcFetcherServer) Fetch(ctx context.Context, req *pb.FetchRequest) (*pb.FetchResponse, error) {
	start := time.Now()

	resp, err := s.httpClient.Get(req.GetUrl())
	if err != nil {
		return &pb.FetchResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return &pb.FetchResponse{}, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return &pb.FetchResponse{}, err
	}

	doc, err := io.ReadAll(resp.Body)
	if err != nil {
		return &pb.FetchResponse{}, err
	}

	fmt.Println(time.Since(start))
	return &pb.FetchResponse{Body: string(doc)}, nil
}

func main() {
	creds := insecure.NewCredentials()
	service, _ := NewFetcherServer()

	server := grpc.NewServer(grpc.Creds(creds), grpc.MaxRecvMsgSize(50*1024*1024))
	pb.RegisterFetcherServiceServer(server, service)
	reflection.Register(server)

	listener, err := net.Listen("tcp", _portNumber)
	if err != nil {
		log.Fatalf("listener: %v", err)
	}

	if err := server.Serve(listener); err != nil {
		log.Fatalf("server: %v", err)
	}

}
