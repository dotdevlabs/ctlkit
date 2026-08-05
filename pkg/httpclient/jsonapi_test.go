package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

type projectAttrs struct {
	Name     string `json:"name"`
	Platform string `json:"platform"`
}

func TestJSONAPISingleDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "projects",
				"id": "42",
				"attributes": {"name": "acme", "platform": "k8s"}
			}
		}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	res, err := httpclient.GetJSONAPISingle[projectAttrs](context.Background(), c, "/projects/42")
	if err != nil {
		t.Fatal(err)
	}
	if res.ID != "42" {
		t.Errorf("ID = %q, want %q", res.ID, "42")
	}
	if res.Type != "projects" {
		t.Errorf("Type = %q, want %q", res.Type, "projects")
	}
	if res.Attributes.Name != "acme" {
		t.Errorf("Attributes.Name = %q, want %q", res.Attributes.Name, "acme")
	}
	if res.Attributes.Platform != "k8s" {
		t.Errorf("Attributes.Platform = %q, want %q", res.Attributes.Platform, "k8s")
	}
}

func TestJSONAPICollectionDecode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"type": "projects", "id": "1", "attributes": {"name": "alpha", "platform": "aws"}},
				{"type": "projects", "id": "2", "attributes": {"name": "beta",  "platform": "gcp"}}
			],
			"links": {
				"first": "/projects?page=1",
				"next":  "/projects?page=2",
				"last":  "/projects?page=5"
			},
			"meta": {"total": 10, "per_page": 2}
		}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	col, err := httpclient.GetJSONAPICollection[projectAttrs](context.Background(), c, "/projects")
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Data) != 2 {
		t.Fatalf("len(Data) = %d, want 2", len(col.Data))
	}
	if col.Data[0].ID != "1" || col.Data[0].Attributes.Name != "alpha" {
		t.Errorf("Data[0] = %+v", col.Data[0])
	}
	if col.Data[1].ID != "2" || col.Data[1].Attributes.Platform != "gcp" {
		t.Errorf("Data[1] = %+v", col.Data[1])
	}
	if col.Links.Next != "/projects?page=2" {
		t.Errorf("Links.Next = %q, want %q", col.Links.Next, "/projects?page=2")
	}
	total, ok := col.Meta["total"].(float64)
	if !ok {
		t.Fatalf("meta[total] type = %T, want float64", col.Meta["total"])
	}
	if total != 10 {
		t.Errorf("meta[total] = %v, want 10", total)
	}
}

func TestJSONAPICollectionEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data": [], "links": {}, "meta": {}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	col, err := httpclient.GetJSONAPICollection[projectAttrs](context.Background(), c, "/projects")
	if err != nil {
		t.Fatal(err)
	}
	if len(col.Data) != 0 {
		t.Errorf("len(Data) = %d, want 0", len(col.Data))
	}
}

func TestJSONAPIErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{
			"errors": [
				{"status": "422", "title": "Invalid", "detail": "name is required"}
			]
		}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.GetJSONAPISingle[projectAttrs](context.Background(), c, "/projects")
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected *clierror.CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeBadRequest {
		t.Errorf("Code = %d, want CodeBadRequest", ce.Code)
	}
	if !strings.Contains(ce.Message, "name is required") {
		t.Errorf("Message = %q, want to contain %q", ce.Message, "name is required")
	}
}

func TestJSONAPIAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data": {"type": "projects", "id": "1", "attributes": {}}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.GetJSONAPISingle[projectAttrs](context.Background(), c, "/projects/1")
	if err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/vnd.api+json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/vnd.api+json")
	}
}

func TestPostJSONAPISingle(t *testing.T) {
	var gotMethod, gotContentType string
	var gotBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{
			"data": {
				"type": "projects",
				"id": "99",
				"attributes": {"name": "new-proj", "platform": "azure"}
			}
		}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	res, err := httpclient.PostJSONAPISingle[projectAttrs](context.Background(), c, "/projects",
		map[string]any{"data": map[string]any{"type": "projects", "attributes": map[string]string{"name": "new-proj"}}})
	if err != nil {
		t.Fatal(err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %q, want POST", gotMethod)
	}
	if gotContentType != "application/vnd.api+json" {
		t.Errorf("Content-Type = %q, want application/vnd.api+json", gotContentType)
	}
	if gotBody == nil {
		t.Error("body was nil")
	}
	if res.ID != "99" {
		t.Errorf("ID = %q, want %q", res.ID, "99")
	}
	if res.Attributes.Name != "new-proj" {
		t.Errorf("Attributes.Name = %q, want %q", res.Attributes.Name, "new-proj")
	}
}

func TestJSONAPIRetryOn503(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data": {"type": "projects", "id": "1", "attributes": {}}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.GetJSONAPISingle[projectAttrs](context.Background(), c, "/projects/1")
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
	}
}

func TestJSONAPIErrorNoDetail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.api+json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{
			"errors": [
				{"status": "404", "title": "Resource Not Found"}
			]
		}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.GetJSONAPISingle[projectAttrs](context.Background(), c, "/projects/999")
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected *clierror.CLIError, got %T", err)
	}
	if !strings.Contains(ce.Message, "Resource Not Found") {
		t.Errorf("Message = %q, want to contain %q", ce.Message, "Resource Not Found")
	}
}

func TestBaseClientAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	var out any
	if err := c.Get(context.Background(), "/items", &out); err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/json")
	}
}

func TestJSONAPICollectionAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data": [], "links": {}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.GetJSONAPICollection[projectAttrs](context.Background(), c, "/projects")
	if err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/vnd.api+json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/vnd.api+json")
	}
}

func TestPostJSONAPISingleAcceptHeader(t *testing.T) {
	var gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		w.Header().Set("Content-Type", "application/vnd.api+json")
		_, _ = w.Write([]byte(`{"data": {"type": "projects", "id": "1", "attributes": {}}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	_, err := httpclient.PostJSONAPISingle[projectAttrs](context.Background(), c, "/projects",
		map[string]any{"data": map[string]any{"type": "projects"}})
	if err != nil {
		t.Fatal(err)
	}
	if gotAccept != "application/vnd.api+json" {
		t.Errorf("Accept = %q, want %q", gotAccept, "application/vnd.api+json")
	}
}
