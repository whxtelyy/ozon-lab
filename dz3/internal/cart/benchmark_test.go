package cart

import (
	"context"
	"testing"

	pb "dz3/internal/gen/product"
)

func benchmarkService() *Service {
	return NewService(&fakeProductClient{products: map[uint32]*pb.Product{
		10: {Sku: 10, Name: "Apple", Price: 100},
		20: {Sku: 20, Name: "Banana", Price: 200},
	}})
}

func BenchmarkAddToCart(b *testing.B) {
	service := benchmarkService()
	ctx := context.Background()
	if err := service.Add(ctx, 1, 10, 1); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := service.Add(ctx, 1, 10, 1); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetCart(b *testing.B) {
	service := benchmarkService()
	for _, sku := range []uint32{20, 10} {
		if err := service.Add(context.Background(), 1, sku, 2); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := service.Get(1); err != nil {
			b.Fatal(err)
		}
	}
}
