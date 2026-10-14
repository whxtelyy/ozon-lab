package docs

import (
	"net/http"
)

const swaggerJSON = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Cart API",
    "version": "1.0.0",
    "description": "REST API сервиса корзины"
  },
  "paths": {
    "/user/{user_id}/cart/{sku_id}": {
      "post": {
        "summary": "Добавить товар в корзину",
        "parameters": [
          {"name":"user_id","in":"path","required":true,"schema":{"type":"integer","format":"int64","minimum":1}},
          {"name":"sku_id","in":"path","required":true,"schema":{"type":"integer","format":"int64","minimum":1}}
        ],
        "requestBody": {
          "required": true,
          "content": {"application/json": {"schema":{"oneOf":[{"type":"integer","format":"uint32","minimum":1},{"type":"object","required":["count"],"properties":{"count":{"type":"integer","format":"uint32","minimum":1}}}]}}}
        },
        "responses": {
          "204":{"description":"Товар добавлен"},
          "400":{"description":"Некорректные user_id, sku_id или count"},
          "412":{"description":"SKU не существует"},
          "500":{"description":"Внутренняя ошибка"}
        }
      }
    },
    "/user/{user_id}/cart": {
      "get": {
        "summary": "Получить корзину пользователя",
        "parameters": [
          {"name":"user_id","in":"path","required":true,"schema":{"type":"integer","format":"int64","minimum":1}}
        ],
        "responses": {
          "200":{"description":"Корзина","content":{"application/json":{"schema":{"$ref":"#/components/schemas/Cart"}}}},
          "400":{"description":"Некорректный user_id"},
          "404":{"description":"Корзина не найдена"},
          "500":{"description":"Внутренняя ошибка"}
        }
      }
    }
  },
  "components": {
    "schemas": {
      "CartItem": {
        "type":"object",
        "properties": {
          "sku":{"type":"integer","format":"int64"},
          "name":{"type":"string"},
          "count":{"type":"integer","format":"uint32"},
          "price":{"type":"integer","format":"uint32"}
        }
      },
      "Cart": {
        "type":"object",
        "properties": {
          "items":{"type":"array","items":{"$ref":"#/components/schemas/CartItem"}},
          "total_price":{"type":"integer","format":"uint32"}
        }
      }
    }
  }
}`

const swaggerHTML = `<!doctype html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <title>Cart API - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: "/swagger.json",
      dom_id: "#swagger-ui"
    });
  </script>
</body>
</html>`

func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "" && r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(swaggerHTML))
	})
}

func JSONHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(swaggerJSON))
	})
}
