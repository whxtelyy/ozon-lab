package main

import (
	"bytes"
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	pb "dz3/internal/gen/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
)

type productStub struct{}

func (productStub) GetProduct(_ context.Context, req *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	return &pb.Product{Sku: req.Sku, Name: "Apple", Price: 100}, nil
}

func TestServeCartOnListener(t *testing.T) {
	t.Parallel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- serve(listener, productStub{}) }()
	client := &http.Client{Timeout: 3 * time.Second}
	t.Cleanup(func() { client.CloseIdleConnections(); listener.Close(); <-done })
	base := "http://" + listener.Addr().String() + "/user/1/cart"
	req, err := http.NewRequest(http.MethodPost, base+"/10", bytes.NewBufferString("2"))
	require.NoError(t, err)
	res, err := client.Do(req)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusNoContent, res.StatusCode)
	res, err = client.Get(base)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
}
