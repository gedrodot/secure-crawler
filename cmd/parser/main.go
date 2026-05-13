package main

import (
	"context"
	"log"
	"net"
	"net/url"
	pb "securecrawler/protos"
	"strings"

	"golang.org/x/net/html"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	_portNumber = ":10117"
)

type GrpcParserServer struct {
	pb.UnimplementedParserServiceServer
}

func NewParserServer() (*GrpcParserServer, error) {
	return &GrpcParserServer{}, nil
}

// Fetch(context.Context, *FetchRequest) (*FetchResponse, error)
func (s *GrpcParserServer) Parse(ctx context.Context, in *pb.ParseRequest) (*pb.ParseResponse, error) {
	doc, err := html.Parse(strings.NewReader(in.Body))
	if err != nil {
		return nil, err
	}

	unirl, err := url.Parse(in.Url)
	if err != nil {
		return nil, err
	}

	var result []string

	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, r := range n.Attr {
				if r.Key == "href" {
					u, err := url.Parse(r.Val)
					if err == nil {
						u = unirl.ResolveReference(u)
						u.RawQuery = ""
						u.Fragment = ""
						u.Path = strings.TrimSuffix(u.Path, "/")

						if u.Host == unirl.Host {
							result = append(result, u.String())
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return &pb.ParseResponse{Urls: result}, nil
}

func main() {
	creds := insecure.NewCredentials()
	service, _ := NewParserServer()

	server := grpc.NewServer(grpc.Creds(creds))
	pb.RegisterParserServiceServer(server, service)
	reflection.Register(server)

	listener, err := net.Listen("tcp", _portNumber)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err) // Громко падаем
	}

	log.Printf("Parser gRPC server is listening on %s", _portNumber)

	if err := server.Serve(listener); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}
