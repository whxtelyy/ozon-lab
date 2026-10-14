//go:build api

package test

import (
	"io"
	"net/http"
	"time"

	pb "dz3/internal/gen/product"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type clients struct {
	product pb.ProductServiceClient
	conn    *grpc.ClientConn
	http    *http.Client
}

func newClients() (*clients, error) {
	conn, err := grpc.NewClient("127.0.0.1:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	return &clients{product: pb.NewProductServiceClient(conn), conn: conn, http: &http.Client{Timeout: 3 * time.Second}}, nil
}

func (c *clients) request(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, "http://localhost:8080"+path, body)
	if err != nil {
		return nil, err
	}
	return c.http.Do(req)
}
