package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	cacheadapter "caching-proxy/internal/adapters/cache"
	"caching-proxy/internal/adapters/origin"
	deliveryhttp "caching-proxy/internal/delivery/http"
	"caching-proxy/internal/usecase"
)

func main() {
	port := flag.Int("port", 3000, "port to listen on")
	originURL := flag.String("origin", "", "origin server URL")
	clearCache := flag.Bool("clear-cache", false, "clear in-memory cache and exit")
	flag.Parse()

	cacheStore := cacheadapter.NewMemoryStore()

	if *clearCache {
		if err := cacheStore.Clear(); err != nil {
			log.Fatalf("clear cache: %v", err)
		}
		fmt.Println("cache cleared")
		return
	}

	if *originURL == "" {
		fmt.Fprintln(os.Stderr, "origin is required: caching-proxy --port 3000 --origin http://dummyjson.com")
		os.Exit(1)
	}

	originClient, err := origin.NewClient(*originURL, http.DefaultClient)
	if err != nil {
		log.Fatalf("create origin client: %v", err)
	}

	proxyUseCase := usecase.NewProxyUseCase(cacheStore, originClient)
	handler := deliveryhttp.NewHandler(proxyUseCase)

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("caching proxy listening on %s, origin %s", addr, *originURL)

	if err := http.ListenAndServe(addr, handler); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
