.PHONY: generate run-product run-cart run-all test test-race test-api bench

generate:
	protoc -I . --go_out=. --go_opt=module=dz3 --go-grpc_out=. --go-grpc_opt=module=dz3 api/product.proto

run-product:
	go run ./cmd/product-service

run-cart:
	go run ./cmd/cart

run-all:
	$(MAKE) -j2 run-product run-cart

test:
	go test ./...

test-race:
	go test -race ./...

test-api:
	go test -v -count=1 -tags=api ./test/

bench:
	go test ./internal/product ./internal/cart -run '^$$' -bench 'Benchmark(GetProduct|AddToCart|GetCart)$$' -benchmem -count=1
