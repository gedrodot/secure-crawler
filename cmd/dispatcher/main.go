package main

import (
	"context"
	"fmt"
	"log"
	"os"
	pb "securecrawler/protos"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func main() {
	start := time.Now()

	fetcherAddr := getEnv("FETCHER_ADDR", "fetcher-service:10116")
	parserAddr := getEnv("PARSER_ADDR", "parser-service:10117")

	baseurl := "http://dari-cveti37.ru"

	var numworkers = 15
	var opts []grpc.DialOption
	var mu sync.RWMutex
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	opts = append(opts, grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(500*1024*1024)))
	urls := make(map[string]bool, 0)

	var wg sync.WaitGroup
	var tasks sync.WaitGroup
	taskch := make(chan string, 100000)
	ctx, cancel := context.WithCancel(context.Background())

	for range numworkers {
		wg.Go(func() {
			connParser, err := grpc.NewClient(parserAddr, opts...)
			if err != nil {
				log.Fatalf("fail to dial: %v", err)
			}
			defer connParser.Close()

			connFetcher, err := grpc.NewClient(fetcherAddr, opts...)
			if err != nil {
				log.Fatalf("fail to dial: %v", err)
			}
			defer connFetcher.Close()

			clientParser := pb.NewParserServiceClient(connParser)
			clientFetcher := pb.NewFetcherServiceClient(connFetcher)

			log.Printf("Getting feature for point")

			for {
				select {
				case <-ctx.Done():
					return
				case url, ok := <-taskch:
					if !ok {
						return
					}
					fetch, err := clientFetcher.Fetch(ctx,
						&pb.FetchRequest{Url: url},
						grpc.MaxCallRecvMsgSize(50*1024*1024))

					if err != nil {
						// ??
						log.Fatalf("fetcher: %v", err)
					}

					parse, err := clientParser.Parse(ctx,
						&pb.ParseRequest{Body: fetch.Body, Url: baseurl},
						grpc.MaxCallRecvMsgSize(50*1024*1024))
					if err != nil {
						log.Fatalf("parser: %v", err)
					}

					mu.Lock()
					for _, u := range parse.Urls {
						ok := !urls[u]
						if ok {

							urls[u] = true

							tasks.Add(1)
							taskch <- u
						}
					}
					mu.Unlock()

					fmt.Println(time.Since(start), len(urls), len(taskch))
					tasks.Done()
				}
			}
		})
	}

	mu.Lock()
	urls[baseurl] = true
	mu.Unlock()
	tasks.Add(1)
	taskch <- baseurl

	tasks.Wait()
	close(taskch)

	cancel()

	wg.Wait()
	fmt.Println(time.Since(start))

	fmt.Println(urls, len(urls))
	select {}
}
