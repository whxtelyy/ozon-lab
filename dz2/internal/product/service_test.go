package product

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "dz2/internal/gen/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const productsJSON = `{"10":{"name":"Apple","price":100},"20":{"name":"Banana","price":200},"30":{"name":"Milk","price":0}}`

func serviceFromJSON(t testing.TB, data string) *Service {
	t.Helper()
	file := filepath.Join(t.TempDir(), "products.json")
	require.NoError(t, os.WriteFile(file, []byte(data), 0600))
	service, err := NewService(file)
	require.NoError(t, err)
	return service
}

func TestGetProduct(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		req  *pb.GetProductRequest
		want *pb.Product
		code codes.Code
	}{
		{"found", &pb.GetProductRequest{Sku: 10}, &pb.Product{Sku: 10, Name: "Apple", Price: 100}, codes.OK},
		{"zero price", &pb.GetProductRequest{Sku: 30}, &pb.Product{Sku: 30, Name: "Milk", Price: 0}, codes.OK},
		{"missing", &pb.GetProductRequest{Sku: 99}, nil, codes.NotFound},
		{"zero SKU", &pb.GetProductRequest{}, nil, codes.InvalidArgument},
		{"nil request", nil, nil, codes.InvalidArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := serviceFromJSON(t, productsJSON)
			got, err := service.GetProduct(context.Background(), tc.req)
			require.Equal(t, tc.code, status.Code(err))
			if tc.code == codes.OK {
				require.Equal(t, tc.want, got)
			} else {
				require.Nil(t, got)
			}
		})
	}
}

func TestListProducts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, data         string
		req                *pb.ListProductsRequest
		page, pages, total uint32
		skus               []uint32
	}{
		{"defaults", productsJSON, nil, 1, 1, 3, []uint32{10, 20, 30}},
		{"first page", productsJSON, &pb.ListProductsRequest{Page: 1, Limit: 2}, 1, 2, 3, []uint32{10, 20}},
		{"last page", productsJSON, &pb.ListProductsRequest{Page: 2, Limit: 2}, 2, 2, 3, []uint32{30}},
		{"out of range", productsJSON, &pb.ListProductsRequest{Page: 3, Limit: 2}, 3, 2, 3, nil},
		{"zero defaults", productsJSON, &pb.ListProductsRequest{}, 1, 1, 3, []uint32{10, 20, 30}},
		{"empty catalog", `{}`, nil, 1, 1, 0, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := serviceFromJSON(t, tc.data)
			got, err := service.ListProducts(context.Background(), tc.req)
			require.NoError(t, err)
			require.Equal(t, tc.page, got.Page)
			require.Equal(t, tc.pages, got.Pages)
			require.Equal(t, tc.total, got.Total)
			skus := make([]uint32, 0, len(got.Products))
			for _, item := range got.Products {
				skus = append(skus, item.Sku)
			}
			if tc.skus == nil {
				require.Empty(t, skus)
			} else {
				require.Equal(t, tc.skus, skus)
			}
		})
	}
}

func TestNewServiceRejectsBadData(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, data string }{
		{"invalid JSON", `[`}, {"zero SKU", `{"0":{"name":"x","price":1}}`},
		{"nonnumeric SKU", `{"x":{"name":"x","price":1}}`},
		{"empty name", `{"1":{"name":"","price":1}}`},
		{"negative price", `{"1":{"name":"x","price":-1}}`},
		{"large price", `{"1":{"name":"x","price":2147483648}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file := filepath.Join(t.TempDir(), "products.json")
			require.NoError(t, os.WriteFile(file, []byte(tc.data), 0600))
			service, err := NewService(file)
			require.Error(t, err)
			require.Nil(t, service)
		})
	}
	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		service, err := NewService(filepath.Join(t.TempDir(), "missing.json"))
		require.Error(t, err)
		require.Nil(t, service)
	})
}

func TestGetProductMaxPrice(t *testing.T) {
	t.Parallel()
	service := serviceFromJSON(t, `{"1":{"name":"Max","price":2147483647}}`)
	got, err := service.GetProduct(context.Background(), &pb.GetProductRequest{Sku: 1})
	require.NoError(t, err)
	require.Equal(t, int32(2147483647), got.Price)
}

func TestListProductsIncludesDetails(t *testing.T) {
	t.Parallel()
	service := serviceFromJSON(t, productsJSON)
	got, err := service.ListProducts(context.Background(), &pb.ListProductsRequest{Page: 1, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []*pb.Product{
		{Sku: 10, Name: "Apple", Price: 100},
		{Sku: 20, Name: "Banana", Price: 200},
	}, got.Products)
}
