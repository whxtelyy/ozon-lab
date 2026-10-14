package main

import (
	"log"
	"net"

	pb "dz2/internal/gen/product"
	"dz2/internal/product"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	service, err := product.NewService("data/products.json")
	if err != nil {
		log.Fatalf("failed to load products: %v", err)
	}

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	log.Println("product-service started on :50051")
	if err := serve(listener, service); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func serve(listener net.Listener, service *product.Service) error {
	server := grpc.NewServer()
	pb.RegisterProductServiceServer(server, service)
	reflection.Register(server)
	return server.Serve(listener)
}
