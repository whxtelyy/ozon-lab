package product

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"

	pb "dz3/internal/gen/product"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Product struct {
	SKU   uint32
	Name  string
	Price uint32
}

type Service struct {
	pb.UnimplementedProductServiceServer

	products map[uint32]Product
}

type jsonProduct struct {
	Name  string `json:"name"`
	Price int64  `json:"price"`
}

func NewService(fileName string) (*Service, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("read products file: %w", err)
	}

	var raw map[string]jsonProduct
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode products file: %w", err)
	}

	products := make(map[uint32]Product, len(raw))
	for skuString, item := range raw {
		sku64, err := strconv.ParseUint(skuString, 10, 32)
		if err != nil || sku64 == 0 {
			return nil, fmt.Errorf("invalid sku %q", skuString)
		}
		if item.Name == "" {
			return nil, fmt.Errorf("product %s has empty name", skuString)
		}
		if item.Price < 0 || item.Price > math.MaxInt32 {
			return nil, fmt.Errorf("product %s has invalid price", skuString)
		}

		sku := uint32(sku64)
		products[sku] = Product{
			SKU:   sku,
			Name:  item.Name,
			Price: uint32(item.Price),
		}
	}

	return &Service{products: products}, nil
}

func (s *Service) GetProduct(_ context.Context, req *pb.GetProductRequest) (*pb.Product, error) {
	if req == nil || req.Sku == 0 {
		return nil, status.Error(codes.InvalidArgument, "SKU должен быть натуральным числом (больше нуля)")
	}

	item, ok := s.products[req.Sku]
	if !ok {
		return nil, status.Error(codes.NotFound, "товар с указанным SKU не найден")
	}

	return &pb.Product{
		Sku:   item.SKU,
		Name:  item.Name,
		Price: int32(item.Price),
	}, nil
}

func (s *Service) ListProducts(_ context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	page := uint64(1)
	limit := uint64(20)
	if req != nil {
		if req.Page > 0 {
			page = uint64(req.Page)
		}
		if req.Limit > 0 {
			limit = uint64(req.Limit)
		}
	}

	keys := make([]uint32, 0, len(s.products))
	for sku := range s.products {
		keys = append(keys, sku)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	total := uint64(len(keys))
	pages := uint64(1)
	if total > 0 {
		pages = (total + limit - 1) / limit
	}

	result := &pb.ListProductsResponse{
		Total: uint32(total),
		Page:  uint32(page),
		Pages: uint32(pages),
	}

	if page > pages || total == 0 {
		return result, nil
	}

	start := (page - 1) * limit
	end := start + limit
	if end > total {
		end = total
	}

	result.Products = make([]*pb.Product, 0, end-start)
	for _, sku := range keys[start:end] {
		item := s.products[sku]
		result.Products = append(result.Products, &pb.Product{
			Sku:   item.SKU,
			Name:  item.Name,
			Price: int32(item.Price),
		})
	}

	return result, nil
}
