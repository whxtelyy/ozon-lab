.PHONY: generate run-product run-cart run-all test

generate:
	protoc -I . --go_out=. --go_opt=module=dz2 --go-grpc_out=. --go-grpc_opt=module=dz2 api/product.proto

run-product:
	go run ./cmd/product-service

run-cart:
	go run ./cmd/cart

run-all:
	$(MAKE) -j2 run-product run-cart

test:
	go test ./...

