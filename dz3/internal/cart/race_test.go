//go:build race

package cart

import (
	"context"
	"fmt"
	"sync"
	"testing"

	pb "dz3/internal/gen/product"
	"github.com/stretchr/testify/require"
)

func TestConcurrentAdds(t *testing.T) {
	t.Parallel()
	const workers = 32
	service := NewService(&fakeProductClient{products: map[uint32]*pb.Product{
		10: {Sku: 10, Name: "Apple", Price: 100},
	}})
	var wg sync.WaitGroup
	errors := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errors <- service.Add(context.Background(), 1, 10, 1)
		}()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	got, err := service.Get(1)
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, uint32(workers), got.Items[0].Count)
	require.Equal(t, uint32(workers*100), got.TotalPrice)
}

func TestConcurrentReadsAndDifferentAdds(t *testing.T) {
	t.Parallel()
	const writers = 16
	const readers = 16
	service := NewService(newFake())
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	var wg sync.WaitGroup
	problems := make(chan error, writers+readers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sku := uint32(10)
			if i%2 == 1 {
				sku = 20
			}
			problems <- service.Add(context.Background(), 1, sku, 1)
		}(i)
	}
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cart, err := service.Get(1)
			if err != nil {
				problems <- err
				return
			}
			var total uint32
			for j, item := range cart.Items {
				if j > 0 && cart.Items[j-1].SKU >= item.SKU {
					problems <- fmt.Errorf("unordered items")
					return
				}
				total += item.Price * item.Count
			}
			if total != cart.TotalPrice {
				problems <- fmt.Errorf("inconsistent total")
			} else {
				problems <- nil
			}
		}()
	}
	wg.Wait()
	close(problems)
	for err := range problems {
		require.NoError(t, err)
	}
	final, err := service.Get(1)
	require.NoError(t, err)
	require.Equal(t, []Item{
		{SKU: 10, Name: "Apple", Count: 9, Price: 100},
		{SKU: 20, Name: "Banana", Count: 8, Price: 200},
	}, final.Items)
	require.Equal(t, uint32(2500), final.TotalPrice)
}
