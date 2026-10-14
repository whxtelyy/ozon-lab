package main

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	pb "dz2/internal/gen/product"
	"dz2/internal/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServeProductOnListener(t *testing.T) {
	t.Parallel()
	file := filepath.Join(t.TempDir(), "products.json")
	require.NoError(t, os.WriteFile(file, []byte(`{"10":{"name":"Apple","price":100}}`), 0600))
	service, err := product.NewService(file)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { done <- serve(listener, service) }()
	t.Cleanup(func() { listener.Close(); <-done })
	conn, err := grpc.NewClient(listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()
	got, err := pb.NewProductServiceClient(conn).GetProduct(ctx, &pb.GetProductRequest{Sku: 10})
	require.NoError(t, err)
	require.Equal(t, uint32(10), got.Sku)
	require.Equal(t, int32(100), got.Price)
}
