package cart

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	pb "dz3/internal/gen/product"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func newHandler() *Handler {
	client := &fakeProductClient{products: map[uint32]*pb.Product{
		10: {Sku: 10, Name: "Apple", Price: 100},
	}}
	return NewHandler(NewService(client))
}

func TestPostCart(t *testing.T) {
	t.Parallel()
	h := newHandler()
	req := httptest.NewRequest(http.MethodPost, "/user/1/cart/10", bytes.NewBufferString("2"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPostCartInvalidArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		body string
	}{
		{"invalid user", "/user/0/cart/10", "1"},
		{"invalid sku", "/user/1/cart/0", "1"},
		{"invalid count", "/user/1/cart/10", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := newHandler()
			req := httptest.NewRequest(http.MethodPost, tt.path, bytes.NewBufferString(tt.body))
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d", w.Code)
			}
		})
	}
}

func TestPostCartMissingSKU(t *testing.T) {
	t.Parallel()
	h := newHandler()
	req := httptest.NewRequest(http.MethodPost, "/user/1/cart/99", bytes.NewBufferString("2"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d", w.Code)
	}
}

func TestGetCart(t *testing.T) {
	t.Parallel()
	h := newHandler()
	addReq := httptest.NewRequest(http.MethodPost, "/user/1/cart/10", bytes.NewBufferString(`{"count":2}`))
	if addReq.Body == nil {
		t.Fatal("missing body")
	}
	addW := httptest.NewRecorder()
	h.ServeHTTP(addW, addReq)
	if addW.Code != http.StatusNoContent {
		t.Fatalf("add failed: %d %s", addW.Code, addW.Body.String())
	}

	getW := httptest.NewRecorder()
	getReq := httptest.NewRequest(http.MethodGet, "/user/1/cart", nil)
	h.ServeHTTP(getW, getReq)
	if getW.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", getW.Code)
	}

	var result Cart
	if err := json.NewDecoder(getW.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Items) != 1 || result.Items[0].Count != 2 || result.TotalPrice != 200 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestGetEmptyCart(t *testing.T) {
	t.Parallel()
	h := newHandler()
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/user/1/cart", nil)
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetCartRESTContract(t *testing.T) {
	t.Parallel()
	t.Run("ordered items and JSON number types", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		service := NewService(fake)
		require.NoError(t, service.Add(context.Background(), 7, 20, 1))
		require.NoError(t, service.Add(context.Background(), 7, 10, 2))
		handler := NewHandler(service)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/7/cart", nil))
		require.Equal(t, http.StatusOK, w.Code)
		require.Equal(t, "application/json", w.Header().Get("Content-Type"))
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		total, ok := body["total_price"].(float64)
		require.True(t, ok)
		require.Equal(t, float64(400), total)
		items, ok := body["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 2)
		first := items[0].(map[string]any)
		second := items[1].(map[string]any)
		require.Equal(t, float64(10), first["sku"])
		require.Equal(t, float64(20), second["sku"])
		require.Equal(t, "Apple", first["name"])
		require.Equal(t, float64(2), first["count"])
		require.Equal(t, float64(100), first["price"])
		require.Equal(t, "Banana", second["name"])
		require.Equal(t, float64(1), second["count"])
		require.Equal(t, float64(200), second["price"])
	})
	t.Run("empty cart", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		w := httptest.NewRecorder()
		NewHandler(NewService(fake)).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/7/cart", nil))
		require.Equal(t, http.StatusNotFound, w.Code)
		require.Zero(t, fake.calls.Load())
	})
	t.Run("dependency failure has no partial success", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		service := NewService(fake)
		require.NoError(t, service.Add(context.Background(), 7, 10, 1))
		require.NoError(t, service.Add(context.Background(), 7, 20, 1))
		fake.failures = map[uint32]error{20: status.Error(codes.Unavailable, "down")}
		w := httptest.NewRecorder()
		NewHandler(service).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/7/cart", nil))
		require.Equal(t, http.StatusPreconditionFailed, w.Code)
		var body map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
		require.Contains(t, body, "error")
		require.NotContains(t, body, "items")
		require.NotContains(t, body, "total_price")
	})
	t.Run("add dependency failure maps to 412", func(t *testing.T) {
		t.Parallel()
		fake := newFake()
		fake.failures = map[uint32]error{10: status.Error(codes.Unavailable, "down")}
		w := httptest.NewRecorder()
		NewHandler(NewService(fake)).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/user/7/cart/10", bytes.NewBufferString("1")))
		require.Equal(t, http.StatusPreconditionFailed, w.Code)
	})
}

func TestConcurrentHTTPAdds(t *testing.T) {
	t.Parallel()
	const requests = 32
	handler := NewHandler(NewService(newFake()))
	statuses := make(chan int, requests)
	var wg sync.WaitGroup
	for i := 0; i < requests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/user/1/cart/10", bytes.NewBufferString("1")))
			statuses <- w.Code
		}()
	}
	wg.Wait()
	close(statuses)
	for code := range statuses {
		require.Equal(t, http.StatusNoContent, code)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/user/1/cart", nil))
	require.Equal(t, http.StatusOK, w.Code)
	var got Cart
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got.Items, 1)
	require.Equal(t, uint32(requests), got.Items[0].Count)
	require.Equal(t, uint32(requests*100), got.TotalPrice)
}
