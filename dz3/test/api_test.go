//go:build api

package test

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	pb "dz3/internal/gen/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestLiveServices(t *testing.T) {
	t.Parallel()
	clients, err := newClients()
	require.NoError(t, err)
	defer clients.conn.Close()
	getProduct := func(sku uint32) (*pb.Product, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return clients.product.GetProduct(ctx, &pb.GetProductRequest{Sku: sku})
	}
	product, err := getProduct(2008)
	require.NoError(t, err)
	require.Equal(t, uint32(2008), product.Sku)
	require.Equal(t, int32(16), product.Price)
	_, err = getProduct(0)
	require.Equal(t, codes.InvalidArgument, status.Code(err))
	_, err = getProduct(999999999)
	require.Equal(t, codes.NotFound, status.Code(err))

	var random [8]byte
	_, err = rand.Read(random[:])
	require.NoError(t, err)
	user := int64(binary.LittleEndian.Uint64(random[:]) >> 1)
	if user == 0 {
		user = 1
	}
	path := fmt.Sprintf("/user/%d/cart", user)
	res, err := clients.request(http.MethodGet, path, nil)
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusNotFound, res.StatusCode)
	for _, add := range []struct {
		sku   uint32
		count string
	}{{3001, "1"}, {2008, "2"}, {2008, "3"}} {
		res, err = clients.request(http.MethodPost, fmt.Sprintf("%s/%d", path, add.sku), bytes.NewBufferString(add.count))
		require.NoError(t, err)
		res.Body.Close()
		require.Equal(t, http.StatusNoContent, res.StatusCode)
	}
	res, err = clients.request(http.MethodPost, path+"/999999999", bytes.NewBufferString("1"))
	require.NoError(t, err)
	res.Body.Close()
	require.Equal(t, http.StatusPreconditionFailed, res.StatusCode)
	res, err = clients.request(http.MethodGet, path, nil)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, http.StatusOK, res.StatusCode)
	var body map[string]any
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	items := body["items"].([]any)
	require.Len(t, items, 2)
	first := items[0].(map[string]any)
	second := items[1].(map[string]any)
	require.Equal(t, float64(2008), first["sku"])
	require.Equal(t, float64(5), first["count"])
	require.Equal(t, float64(16), first["price"])
	require.Equal(t, float64(3001), second["sku"])
	require.Equal(t, float64(4999), second["price"])
	require.Equal(t, float64(5079), body["total_price"])
}
