package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jiangchengyu998/demo-go/internal/item"
	"github.com/jiangchengyu998/demo-go/internal/observability"
)

type Handler struct {
	service *item.Service
	mux     *http.ServeMux
}

func NewHandler(service *item.Service) http.Handler {
	handler := &Handler{service: service, mux: http.NewServeMux()}
	handler.routes()
	return handler.mux
}

func (h *Handler) routes() {
	h.mux.HandleFunc("/", h.index)
	h.mux.HandleFunc("/api/items", h.items)
	h.mux.HandleFunc("/api/items/", h.itemByID)
	h.mux.HandleFunc("/actuator/health", h.health)
	h.mux.Handle("/actuator/prometheus", observability.PrometheusHandler())
	h.mux.HandleFunc("/swagger-ui.html", h.swaggerUI)
	h.mux.HandleFunc("/v3/api-docs", h.openAPI)
}

func (h *Handler) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		writeError(w, http.StatusNotFound, "Not Found", "Resource not found", nil)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexHTML))
}

func (h *Handler) items(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listItems(w, r)
	case http.MethodPost:
		h.createItem(w, r)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Method not allowed", nil)
	}
}

func (h *Handler) itemByID(w http.ResponseWriter, r *http.Request) {
	id, ok := parseItemID(w, r.URL.Path)
	if !ok {
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getItem(w, r, id)
	case http.MethodPut:
		h.updateItem(w, r, id)
	case http.MethodDelete:
		h.deleteItem(w, r, id)
	default:
		w.Header().Set("Allow", "GET, PUT, DELETE")
		writeError(w, http.StatusMethodNotAllowed, "Method Not Allowed", "Method not allowed", nil)
	}
}

func (h *Handler) listItems(w http.ResponseWriter, r *http.Request) {
	page, ok := intQuery(w, r, "page", 0)
	if !ok {
		return
	}
	size, ok := intQuery(w, r, "size", 20)
	if !ok {
		return
	}

	response, err := h.service.List(r.Context(), page, size, r.URL.Query().Get("sort"))
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pageResponseFromItemPage(response))
}

func (h *Handler) getItem(w http.ResponseWriter, r *http.Request, id int64) {
	response, err := h.service.Get(r.Context(), id)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, itemResponseFromItem(response))
}

func (h *Handler) createItem(w http.ResponseWriter, r *http.Request) {
	request, ok := decodeItemRequest(w, r)
	if !ok {
		return
	}

	response, err := h.service.Create(r.Context(), request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, itemResponseFromItem(response))
}

func (h *Handler) updateItem(w http.ResponseWriter, r *http.Request, id int64) {
	request, ok := decodeItemRequest(w, r)
	if !ok {
		return
	}

	response, err := h.service.Update(r.Context(), id, request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, itemResponseFromItem(response))
}

func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request, id int64) {
	if err := h.service.Delete(r.Context(), id); err != nil {
		writeServiceError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "UP"})
}

func (h *Handler) swaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerHTML))
}

func (h *Handler) openAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(openAPIJSON))
}

func decodeItemRequest(w http.ResponseWriter, r *http.Request) (item.Request, bool) {
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	var payload itemRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		writeError(w, http.StatusBadRequest, "Bad Request", "Request validation failed", []string{err.Error()})
		return item.Request{}, false
	}
	return item.Request{Name: payload.Name, Description: payload.Description}, true
}

func parseItemID(w http.ResponseWriter, path string) (int64, bool) {
	value := strings.TrimPrefix(path, "/api/items/")
	if value == "" || strings.Contains(value, "/") {
		writeError(w, http.StatusNotFound, "Not Found", "Resource not found", nil)
		return 0, false
	}
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id < 1 {
		writeError(w, http.StatusBadRequest, "Bad Request", "Request validation failed", []string{"id: must be a positive integer"})
		return 0, false
	}
	return id, true
}

func intQuery(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Bad Request", "Request validation failed", []string{fmt.Sprintf("%s: must be an integer", name)})
		return 0, false
	}
	return value, true
}

func writeServiceError(w http.ResponseWriter, err error) {
	var validation item.ValidationError
	switch {
	case errors.As(err, &validation):
		writeError(w, http.StatusBadRequest, "Bad Request", "Request validation failed", validation.Details)
	case errors.Is(err, item.ErrNotFound):
		writeError(w, http.StatusNotFound, "Not Found", err.Error(), nil)
	default:
		writeError(w, http.StatusInternalServerError, "Internal Server Error", "Internal server error", nil)
	}
}

func writeError(w http.ResponseWriter, status int, reason, message string, details []string) {
	if details == nil {
		details = []string{}
	}
	writeJSON(w, status, apiError{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Status:    status,
		Error:     reason,
		Message:   message,
		Details:   details,
	})
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
