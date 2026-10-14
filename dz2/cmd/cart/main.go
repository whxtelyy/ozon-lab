package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"dz2/docs"
	"dz2/internal/cart"
	pb "dz2/internal/gen/product"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.NewClient("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("failed to connect to product-service: %v", err)
	}
	defer conn.Close()

	productClient := pb.NewProductServiceClient(conn)
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Println("cart started on :8080")
	log.Println("swagger: http://localhost:8080/swagger/")
	if err := serve(listener, productClient); err != nil && err != http.ErrServerClosed {
		log.Fatalf("failed to serve: %v", err)
	}
}

func serve(listener net.Listener, productClient cart.ProductGetter) error {
	cartService := cart.NewService(productClient)
	handler := cart.LoggingMiddleware(cart.NewHandler(cartService))
	mux := http.NewServeMux()
	mux.Handle("/swagger/", http.StripPrefix("/swagger/", docs.Handler()))
	mux.Handle("/swagger.json", docs.JSONHandler())
	mux.Handle("/", handler)

	server := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return server.Serve(listener)
}
