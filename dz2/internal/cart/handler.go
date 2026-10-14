package cart

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	parts := strings.Split(path, "/")

	if r.Method == http.MethodPost && len(parts) == 4 && parts[0] == "user" && parts[2] == "cart" {
		h.add(w, r, parts[1], parts[3])
		return
	}
	if r.Method == http.MethodGet && len(parts) == 3 && parts[0] == "user" && parts[2] == "cart" {
		h.get(w, r, parts[1])
		return
	}

	writeError(w, http.StatusNotFound, "маршрут не найден")
}

func (h *Handler) add(w http.ResponseWriter, r *http.Request, userIDText, skuText string) {
	userID, err := strconv.ParseInt(userIDText, 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "Идентификатор пользователя должен быть натуральным числом (больше нуля)")
		return
	}

	sku64, err := strconv.ParseUint(skuText, 10, 32)
	if err != nil || sku64 == 0 {
		writeError(w, http.StatusBadRequest, "SKU должен быть натуральным числом (больше нуля)")
		return
	}

	count, err := readCount(r)
	if err != nil || count == 0 {
		writeError(w, http.StatusBadRequest, "Количество должно быть натуральным числом (больше нуля)")
		return
	}

	err = h.service.Add(r.Context(), userID, uint32(sku64), count)
	if err != nil {
		if errors.Is(err, ErrProductUnavailable) || status.Code(err) == codes.NotFound {
			writeError(w, http.StatusPreconditionFailed, "SKU должен существовать в сервисе product-service")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request, userIDText string) {
	userID, err := strconv.ParseInt(userIDText, 10, 64)
	if err != nil || userID <= 0 {
		writeError(w, http.StatusBadRequest, "Идентификатор пользователя должен быть натуральным числом (больше нуля)")
		return
	}

	result, err := h.service.GetWithContext(r.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrCartNotFound) {
			writeError(w, http.StatusNotFound, "корзина не найдена")
			return
		}
		if errors.Is(err, ErrProductUnavailable) {
			writeError(w, http.StatusPreconditionFailed, "не удалось получить товар из product-service")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func readCount(r *http.Request) (uint32, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1024))
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return 0, fmt.Errorf("empty body")
	}

	var count uint32
	if err := json.Unmarshal(body, &count); err == nil {
		return count, nil
	}

	var object struct {
		Count *uint32 `json:"count"`
	}
	if err := json.Unmarshal(body, &object); err != nil || object.Count == nil {
		return 0, fmt.Errorf("count must be uint32")
	}
	return *object.Count, nil
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		return
	}
}

func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
