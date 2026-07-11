package httpapi_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jiangchengyu998/demo-go/internal/httpapi"
	"github.com/jiangchengyu998/demo-go/internal/item"
	"github.com/jiangchengyu998/demo-go/internal/storage"
)

func TestCRUDFlow(t *testing.T) {
	client, baseURL := testServer(t)

	created := doJSON(t, client, http.MethodPost, baseURL+"/api/items", `{"name":"demo item","description":"created from go test"}`)
	if created.status != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", created.status, created.body)
	}
	if created.json["id"].(float64) != 1 {
		t.Fatalf("created id = %v", created.json["id"])
	}
	if created.json["name"] != "demo item" {
		t.Fatalf("created name = %v", created.json["name"])
	}
	if created.json["description"] != "created from go test" {
		t.Fatalf("created description = %v", created.json["description"])
	}
	if created.json["createdAt"] == "" || created.json["updatedAt"] == "" {
		t.Fatalf("created timestamps missing: %#v", created.json)
	}

	listed := doJSON(t, client, http.MethodGet, baseURL+"/api/items", "")
	if listed.status != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listed.status, listed.body)
	}
	if listed.json["totalElements"].(float64) != 1 || listed.json["totalPages"].(float64) != 1 {
		t.Fatalf("list page counters = %#v", listed.json)
	}
	if listed.json["first"] != true || listed.json["last"] != true {
		t.Fatalf("list flags = %#v", listed.json)
	}

	fetched := doJSON(t, client, http.MethodGet, baseURL+"/api/items/1", "")
	if fetched.status != http.StatusOK || fetched.json["name"] != "demo item" {
		t.Fatalf("fetch response = %d %#v", fetched.status, fetched.json)
	}

	updated := doJSON(t, client, http.MethodPut, baseURL+"/api/items/1", `{"name":"updated item","description":null}`)
	if updated.status != http.StatusOK {
		t.Fatalf("update status = %d, body = %s", updated.status, updated.body)
	}
	if updated.json["name"] != "updated item" || updated.json["description"] != nil {
		t.Fatalf("updated body = %#v", updated.json)
	}

	deleted := doJSON(t, client, http.MethodDelete, baseURL+"/api/items/1", "")
	if deleted.status != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleted.status, deleted.body)
	}

	missing := doJSON(t, client, http.MethodGet, baseURL+"/api/items/1", "")
	if missing.status != http.StatusNotFound {
		t.Fatalf("missing status = %d, body = %s", missing.status, missing.body)
	}
	if missing.json["message"] != "Item not found: 1" {
		t.Fatalf("missing message = %#v", missing.json)
	}
	if len(missing.json["details"].([]any)) != 0 {
		t.Fatalf("missing details = %#v", missing.json)
	}
}

func TestValidationErrorMatchesAPIErrorShape(t *testing.T) {
	client, baseURL := testServer(t)

	response := doJSON(t, client, http.MethodPost, baseURL+"/api/items", `{"name":"   "}`)
	if response.status != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", response.status, response.body)
	}
	if response.json["status"].(float64) != 400 ||
		response.json["error"] != "Bad Request" ||
		response.json["message"] != "Request validation failed" {
		t.Fatalf("error shape = %#v", response.json)
	}
	if len(response.json["details"].([]any)) == 0 {
		t.Fatalf("details missing = %#v", response.json)
	}
}

func TestHealthAndOpenAPI(t *testing.T) {
	client, baseURL := testServer(t)

	health := doJSON(t, client, http.MethodGet, baseURL+"/actuator/health", "")
	if health.status != http.StatusOK || health.json["status"] != "UP" {
		t.Fatalf("health = %d %#v", health.status, health.json)
	}

	openAPI := doJSON(t, client, http.MethodGet, baseURL+"/v3/api-docs", "")
	if openAPI.status != http.StatusOK {
		t.Fatalf("openapi status = %d, body = %s", openAPI.status, openAPI.body)
	}
	if openAPI.json["openapi"] != "3.0.3" {
		t.Fatalf("openapi body = %#v", openAPI.json)
	}
}

func testServer(t *testing.T) (*http.Client, string) {
	t.Helper()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	service := item.NewService(storage.NewMemoryRepository(), logger)
	server := httptest.NewServer(httpapi.NewHandler(service))
	t.Cleanup(server.Close)
	return server.Client(), server.URL
}

type jsonResponse struct {
	status int
	body   string
	json   map[string]any
}

func doJSON(t *testing.T, client *http.Client, method, url, body string) jsonResponse {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = bytes.NewBufferString(body)
	}
	request, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}

	result := jsonResponse{status: response.StatusCode, body: string(raw)}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &result.json); err != nil {
			t.Fatalf("decode JSON %q: %v", raw, err)
		}
	}
	return result
}
