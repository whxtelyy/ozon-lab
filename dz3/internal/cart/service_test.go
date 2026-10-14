package cart

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	pb "dz3/internal/gen/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type fakeProductClient struct {
	products map[uint32]*pb.Product
	failures map[uint32]error
	calls    atomic.Int64
	onGet    func(context.Context, uint32)
}

func (f *fakeProductClient) GetProduct(ctx context.Context, req *pb.GetProductRequest, _ ...grpc.CallOption) (*pb.Product, error) {
	f.calls.Add(1)
	if f.onGet != nil {
		f.onGet(ctx, req.Sku)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := f.failures[req.Sku]; err != nil {
		return nil, err
	}
	product, ok := f.products[req.Sku]
	if !ok {
		return nil, status.Error(codes.NotFound, "not found")
	}
	return product, nil
}

func newFake() *fakeProductClient {
	return &fakeProductClient{products: map[uint32]*pb.Product{
		10: {Sku: 10, Name: "Apple", Price: 100},
		20: {Sku: 20, Name: "Banana", Price: 200},
		30: {Sku: 30, Name: "Milk", Price: 0},
	}}
}

func TestCartBusinessRules(t *testing.T) {
	t.Parallel()
	type addInput struct {
		user       int64
		sku, count uint32
	}
	cases := []struct {
		name    string
		adds    []addInput
		user    int64
		want    Cart
		wantErr error
	}{
		{"one product", []addInput{{1, 10, 2}}, 1,
			Cart{Items: []Item{{SKU: 10, Name: "Apple", Count: 2, Price: 100}}, TotalPrice: 200}, nil},
		{"multiple and repeat", []addInput{{1, 20, 1}, {1, 10, 2}, {1, 10, 3}}, 1,
			Cart{Items: []Item{{SKU: 10, Name: "Apple", Count: 5, Price: 100}, {SKU: 20, Name: "Banana", Count: 1, Price: 200}}, TotalPrice: 700}, nil},
		{"isolated users", []addInput{{1, 10, 5}, {2, 20, 2}}, 2,
			Cart{Items: []Item{{SKU: 20, Name: "Banana", Count: 2, Price: 200}}, TotalPrice: 400}, nil},
		{"zero price", []addInput{{1, 30, 2}}, 1,
			Cart{Items: []Item{{SKU: 30, Name: "Milk", Count: 2, Price: 0}}, TotalPrice: 0}, nil},
		{"empty", nil, 1, Cart{}, ErrCartNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			service := NewService(newFake())
			for _, add := range tc.adds {
				require.NoError(t, service.Add(context.Background(), add.user, add.sku, add.count))
			}
			got, err := service.Get(tc.user)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestCartInvalidAdd(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		user       int64
		sku, count uint32
		calls      int64
	}{
		{"invalid user", 0, 10, 1, 0}, {"zero SKU", 1, 0, 1, 0}, {"zero count", 1, 10, 0, 0},
		{"missing product", 1, 99, 1, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := newFake()
			service := NewService(fake)
			require.Error(t, service.Add(context.Background(), tc.user, tc.sku, tc.count))
			require.Equal(t, tc.calls, fake.calls.Load())
			_, err := service.Get(1)
			require.ErrorIs(t, err, ErrCartNotFound)
		})
	}
}

func TestCartDependencyFailures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		product *pb.Product
		err     error
	}{
		{"gRPC unavailable", nil, status.Error(codes.Unavailable, "down")},
		{"nil response", nil, nil},
		{"negative price", &pb.Product{Sku: 10, Price: -1}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fake := newFake()
			fake.products[10] = tc.product
			fake.failures = map[uint32]error{10: tc.err}
			service := NewService(fake)
			err := service.Add(context.Background(), 1, 10, 1)
			require.ErrorIs(t, err, ErrProductUnavailable)
			if tc.err != nil {
				require.Equal(t, codes.Unavailable, status.Code(err))
			}
			_, err = service.Get(1)
			require.ErrorIs(t, err, ErrCartNotFound)
		})
	}
}

func TestCartMissingProductCode(t *testing.T) {
	t.Parallel()
	service := NewService(newFake())
	err := service.Add(context.Background(), 1, 99, 1)
	require.ErrorIs(t, err, ErrProductUnavailable)
	require.Equal(t, codes.NotFound, status.Code(err))
}

func TestGetCartUsesCurrentProductDetails(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 2))
	fake.products[10] = &pb.Product{Sku: 10, Name: "Updated", Price: 150}
	got, err := service.Get(1)
	require.NoError(t, err)
	require.Equal(t, []Item{{SKU: 10, Name: "Updated", Count: 2, Price: 150}}, got.Items)
	require.Equal(t, uint32(300), got.TotalPrice)
}

func TestCartLimitsAndSnapshot(t *testing.T) {
	t.Parallel()
	t.Run("invalid get user", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		_, err := NewService(fake).Get(0)
		require.Error(t, err)
		require.Zero(t, fake.calls.Load())
	})
	t.Run("count overflow retains state", func(t *testing.T) {
		t.Parallel()
		service := NewService(newFake())
		require.NoError(t, service.Add(context.Background(), 1, 10, math.MaxUint32))
		require.Error(t, service.Add(context.Background(), 1, 10, 1))
		// Get cannot represent a total above uint32; inspect the protected state.
		service.mu.RLock()
		count := service.carts[1][10].Count
		service.mu.RUnlock()
		require.Equal(t, uint32(math.MaxUint32), count)
	})
	t.Run("total overflow", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		fake.products[10].Price = math.MaxInt32
		service := NewService(fake)
		require.NoError(t, service.Add(context.Background(), 1, 10, 3))
		_, err := service.Get(1)
		require.ErrorContains(t, err, "общая стоимость")
	})
	t.Run("result is detached", func(t *testing.T) {
		t.Parallel()
		service := NewService(newFake())
		require.NoError(t, service.Add(context.Background(), 1, 10, 1))
		first, err := service.Get(1)
		require.NoError(t, err)
		first.Items[0].Count = 99
		second, err := service.Get(1)
		require.NoError(t, err)
		require.Equal(t, uint32(1), second.Items[0].Count)
	})
}

func TestGetCartRequestsInParallel(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	require.NoError(t, service.Add(context.Background(), 1, 20, 1))
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	fake.onGet = func(ctx context.Context, _ uint32) {
		entered <- struct{}{}
		select {
		case <-release:
		case <-ctx.Done():
		}
	}
	done := make(chan error, 1)
	go func() { _, err := service.GetWithContext(context.Background(), 1); done <- err }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("first request did not start")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("product requests were not parallel")
	}
	releaseOnce.Do(func() { close(release) })
	require.NoError(t, <-done)
}

func TestGetCartSharedDeadline(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	require.NoError(t, service.Add(context.Background(), 1, 20, 1))
	deadlines := make(chan time.Time, 2)
	fake.onGet = func(ctx context.Context, _ uint32) {
		deadline, ok := ctx.Deadline()
		if ok {
			deadlines <- deadline
		}
	}
	start := time.Now()
	_, err := service.GetWithContext(context.Background(), 1)
	require.NoError(t, err)
	require.Equal(t, int64(4), fake.calls.Load())
	require.Len(t, deadlines, 2)
	first, second := <-deadlines, <-deadlines
	require.Equal(t, first, second)
	require.WithinDuration(t, start.Add(productBatchTimeout), first, time.Second)
}

func TestGetCartHonorsCallerDeadline(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	fake.onGet = func(ctx context.Context, _ uint32) { <-ctx.Done() }
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := service.GetWithContext(ctx, 1)
	require.ErrorIs(t, err, ErrProductUnavailable)
	require.Equal(t, int64(2), fake.calls.Load())
}

func TestGetCartKeepsOrderWhenProductsFinishOutOfOrder(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	require.NoError(t, service.Add(context.Background(), 1, 20, 1))
	fastStarted := make(chan struct{})
	fake.onGet = func(ctx context.Context, sku uint32) {
		if sku == 20 {
			close(fastStarted)
			return
		}
		select {
		case <-fastStarted:
		case <-ctx.Done():
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	got, err := service.Get(1)
	require.NoError(t, err)
	require.Len(t, got.Items, 2)
	require.Equal(t, []uint32{10, 20}, []uint32{got.Items[0].SKU, got.Items[1].SKU})
	require.Equal(t, uint32(300), got.TotalPrice)
}

func TestGetCartCancelsOutstandingRequestsOnFailure(t *testing.T) {
	t.Parallel()
	fake := newFake()
	service := NewService(fake)
	require.NoError(t, service.Add(context.Background(), 1, 10, 1))
	require.NoError(t, service.Add(context.Background(), 1, 20, 1))
	fake.failures = map[uint32]error{20: status.Error(codes.Unavailable, "down")}
	fake.onGet = func(ctx context.Context, sku uint32) {
		if sku == 10 {
			<-ctx.Done()
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := service.GetWithContext(ctx, 1); done <- err }()
	select {
	case err := <-done:
		require.ErrorIs(t, err, ErrProductUnavailable)
		require.Equal(t, codes.Unavailable, status.Code(err))
	case <-time.After(time.Second):
		t.Fatal("remaining product request was not canceled after failure")
	}
	require.Equal(t, int64(4), fake.calls.Load())
}

func TestGetEmptyMakesNoProductCalls(t *testing.T) {
	t.Parallel()
	fake := newFake()
	_, err := NewService(fake).Get(1)
	require.ErrorIs(t, err, ErrCartNotFound)
	require.Zero(t, fake.calls.Load())
}
