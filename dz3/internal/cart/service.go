package cart

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	pb "dz3/internal/gen/product"
	"google.golang.org/grpc"
)

var ErrCartNotFound = errors.New("cart not found")
var ErrProductUnavailable = errors.New("product unavailable")

const productBatchTimeout = 3 * time.Second

type ProductGetter interface {
	GetProduct(context.Context, *pb.GetProductRequest, ...grpc.CallOption) (*pb.Product, error)
}

type Item struct {
	SKU   uint32 `json:"sku"`
	Name  string `json:"name"`
	Count uint32 `json:"count"`
	Price uint32 `json:"price"`
}

type Cart struct {
	Items      []Item `json:"items"`
	TotalPrice uint32 `json:"total_price"`
}

type Service struct {
	products ProductGetter
	mu       sync.RWMutex
	carts    map[int64]map[uint32]Item
}

func NewService(products ProductGetter) *Service {
	return &Service{
		products: products,
		carts:    make(map[int64]map[uint32]Item),
	}
}

func (s *Service) Add(ctx context.Context, userID int64, sku uint32, count uint32) error {
	if userID <= 0 {
		return fmt.Errorf("userID должен быть натуральным числом (больше нуля)")
	}
	if sku == 0 {
		return fmt.Errorf("SKU должен быть натуральным числом (больше нуля)")
	}
	if count == 0 {
		return fmt.Errorf("Количество должно быть натуральным числом (больше нуля)")
	}

	product, err := s.products.GetProduct(ctx, &pb.GetProductRequest{Sku: sku})
	if err != nil {
		return fmt.Errorf("%w: %w", ErrProductUnavailable, err)
	}
	if product == nil {
		return fmt.Errorf("%w: пустой ответ от product-service", ErrProductUnavailable)
	}
	if product.Price < 0 {
		return fmt.Errorf("%w: некорректная цена", ErrProductUnavailable)
	}

	item := Item{
		SKU:   sku,
		Name:  product.Name,
		Count: count,
		Price: uint32(product.Price),
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	userCart, ok := s.carts[userID]
	if !ok {
		userCart = make(map[uint32]Item)
		s.carts[userID] = userCart
	}

	old, ok := userCart[sku]
	if ok {
		if uint64(old.Count)+uint64(count) > math.MaxUint32 {
			return fmt.Errorf("количество товара слишком большое")
		}
		item.Count = old.Count + count
		item.Name = old.Name
		item.Price = old.Price
	}

	userCart[sku] = item
	return nil
}

func (s *Service) Get(userID int64) (Cart, error) {
	return s.GetWithContext(context.Background(), userID)
}

func (s *Service) GetWithContext(ctx context.Context, userID int64) (Cart, error) {
	if userID <= 0 {
		return Cart{}, fmt.Errorf("userID должен быть натуральным числом (больше нуля)")
	}

	s.mu.RLock()
	userCart, ok := s.carts[userID]
	if !ok || len(userCart) == 0 {
		s.mu.RUnlock()
		return Cart{}, ErrCartNotFound
	}

	// Копия позволяет отпустить блокировку до сетевых запросов.
	snapshot := make([]Item, 0, len(userCart))
	for _, item := range userCart {
		snapshot = append(snapshot, item)
	}
	s.mu.RUnlock()

	sort.Slice(snapshot, func(i, j int) bool { return snapshot[i].SKU < snapshot[j].SKU })
	batchCtx, cancel := context.WithTimeout(ctx, productBatchTimeout)
	defer cancel()
	items := make([]Item, len(snapshot))
	var wg sync.WaitGroup
	var failOnce sync.Once
	var firstErr error
	// Первая ошибка отменяет остальные запросы и сохраняет её причину.
	fail := func(err error) {
		failOnce.Do(func() {
			firstErr = err
			cancel()
		})
	}
	for i, item := range snapshot {
		wg.Add(1)
		go func(i int, item Item) {
			defer wg.Done()
			product, err := s.products.GetProduct(batchCtx, &pb.GetProductRequest{Sku: item.SKU})
			if err != nil {
				fail(fmt.Errorf("%w: %w", ErrProductUnavailable, err))
				return
			}
			if product == nil || product.Price < 0 {
				fail(fmt.Errorf("%w: некорректный ответ", ErrProductUnavailable))
				return
			}
			// Каждая горутина пишет только в свой индекс.
			items[i] = Item{SKU: item.SKU, Name: product.Name, Count: item.Count, Price: uint32(product.Price)}
		}(i, item)
	}
	wg.Wait()
	if firstErr != nil {
		return Cart{}, firstErr
	}

	result := Cart{Items: items}
	for _, item := range items {
		lineTotal := uint64(item.Price) * uint64(item.Count)
		if uint64(result.TotalPrice)+lineTotal > math.MaxUint32 {
			return Cart{}, fmt.Errorf("общая стоимость корзины слишком большая")
		}
		result.TotalPrice += uint32(lineTotal)
	}
	return result, nil
}
