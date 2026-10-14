# ДЗ2 - Микросервисы и gRPC

Два сервиса на Go: `product-service` предоставляет товары через gRPC, а `cart` управляет корзинами пользователей через HTTP. Данные загружаются из JSON и хранятся в памяти; после перезапуска корзины очищаются.

## Возможности

- `GetProduct` и `ListProducts` с пагинацией в gRPC API товаров.
- Добавление товаров в корзину и получение ее содержимого через HTTP.
- Проверка SKU через gRPC при добавлении товара.
- Суммирование количества одинаковых товаров и расчет `total_price`.
- Сортировка позиций корзины по SKU.
- Логирование HTTP-запросов и Swagger/OpenAPI.

## Структура

```text
api/product.proto         gRPC-контракт
cmd/product-service/      запуск сервиса товаров
cmd/cart/                 запуск HTTP-сервиса
internal/product/         логика товаров и unit-тесты
internal/cart/            логика корзины, HTTP-обработчик и unit-тесты
internal/gen/product/     сгенерированный protobuf-код
data/products.json        каталог товаров
docs/                     OpenAPI и Swagger UI
Makefile                  команды запуска, генерации и тестирования
```

## Запуск

Нужны Go 1.23 и `make`. Команды выполняются из `dz2/`:

```bash
make run-all
```

Оба сервиса можно запустить по отдельности в разных терминалах:

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

В POST можно передать число или JSON-объект `{"count":2}`. Успешное добавление возвращает HTTP `204`. GET возвращает позиции по возрастанию SKU:

```json
{
  "items": [
    {"sku": 2008, "name": "Гречка пропаренная, в пакетиках для варки, 400", "count": 2, "price": 16}
  ],
  "total_price": 32
}
```

Неверные параметры возвращают `400`, отсутствующая корзина - `404`, несуществующий SKU при добавлении - `412`.

В gRPC доступны `ProductService/GetProduct` и `ProductService/ListProducts`. Для некорректного SKU возвращается `InvalidArgument`, для отсутствующего - `NotFound`.

Swagger UI: [http://localhost:8080/swagger/](http://localhost:8080/swagger/). OpenAPI JSON: [http://localhost:8080/swagger.json](http://localhost:8080/swagger.json).

## Генерация protobuf

Для генерации нужны `protoc`, `protoc-gen-go` и `protoc-gen-go-grpc`:

```bash
make generate
```

## Тестирование

```bash
gofmt -l .
go test ./...
go build ./...
go vet ./...
```

Unit-тесты находятся в пакетах `internal/product` и `internal/cart`.
