# ДЗ3 - Тестирование и конкурентность

Продолжение ДЗ2 с теми же `product-service` и `cart`, но с дополнительными тестами и поддержкой конкурентных запросов. Сервисы работают независимо от ДЗ2 и используют собственный Go-модуль.

## Возможности

- gRPC API товаров: `GetProduct` и `ListProducts` с пагинацией.
- HTTP API корзины с проверкой SKU через gRPC.
- Потокобезопасное хранение корзин с `sync.RWMutex`.
- Параллельное получение данных о товарах при чтении корзины.
- Unit-тесты, интеграционные тесты, race-тесты и бенчмарки.
- Логирование HTTP-запросов и Swagger/OpenAPI.

## Структура

```text
api/product.proto         gRPC-контракт
cmd/product-service/      запуск сервиса товаров
cmd/cart/                 запуск HTTP-сервиса
internal/product/         логика товаров и unit-тесты
internal/cart/            логика корзины, HTTP-тесты, race-тесты и бенчмарки
internal/gen/product/     сгенерированный protobuf-код
data/products.json        каталог товаров
docs/                     OpenAPI, Swagger UI и результаты измерений
test/                     black-box интеграционные тесты (build tag api)
Makefile                  команды запуска, генерации и тестирования
```

## Запуск

Нужны Go 1.23 и `make`. Команды выполняются из `dz3/`:

```bash
make run-all
```

Либо запусти сервисы в разных терминалах:

```bash
make run-product
make run-cart
```

`product-service` слушает gRPC на `localhost:50051`, `cart` - HTTP на `localhost:8080`.

## API

Добавить товар в корзину:

```bash
curl -X POST http://localhost:8080/user/1007/cart/2008 -H "Content-Type: application/json" -d '2'
```

Получить корзину:

```bash
curl http://localhost:8080/user/1007/cart
```

POST также принимает `{"count":2}`. В GET товары отсортированы по SKU; ответ содержит `items` с полями `sku`, `name`, `count`, `price` и общую стоимость `total_price`. В gRPC доступны методы `GetProduct` и `ListProducts`.

Swagger UI: [http://localhost:8080/swagger/](http://localhost:8080/swagger/). OpenAPI JSON: [http://localhost:8080/swagger.json](http://localhost:8080/swagger.json).

## Конкурентность

Для чтения и изменения корзин используется `sync.RWMutex`. При получении корзины ее содержимое копируется под `RLock`, после чего блокировка освобождается до выполнения сетевых запросов.

Данные о товарах запрашиваются параллельно: по одной горутине на SKU, с `sync.WaitGroup` и общим таймаутом 3 секунды. При ошибке запросы отменяются, неполный успешный ответ не возвращается. Порядок товаров по SKU сохраняется.

## Генерация protobuf

Для генерации нужны `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc`:

```bash
make generate
```

## Тестирование

Стандартные проверки:

```bash
gofmt -l .
go test ./...
go build ./...
go vet ./...
```

Проверка гонок и бенчмарки:

```bash
make test-race
make bench
```

Для `make test-race` нужны включенный CGO и C-компилятор. Бенчмарки запускаются с `-benchmem` и измеряют `GetProduct`, `AddToCart` и `GetCart`.

Интеграционные тесты находятся в `test/` и используют build tag `api`. Сначала запусти оба сервиса через `make run-all`, затем в отдельном терминале выполни:

```bash
make test-api
```

Эти тесты обращаются к сервисам через реальные HTTP- и gRPC-запросы и не входят в обычный `go test ./...`.

Описание замеров и сравнительные результаты находятся в [`docs/testing-results.md`](./docs/testing-results.md). Для демонстрации гонки использовалась отдельная временная копия без блокировок; это не подтверждение гонки в исторической версии кода.