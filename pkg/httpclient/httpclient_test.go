package httpclient_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

func TestBrowserUA(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	var out any
	if err := c.Get(context.Background(), "/", &out); err != nil {
		t.Fatal(err)
	}
	if gotUA != httpclient.BrowserUserAgent {
		t.Errorf("User-Agent = %q, want %q", gotUA, httpclient.BrowserUserAgent)
	}
}

func TestBearerAuth(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "secret-token")
	var out any
	if err := c.Get(context.Background(), "/", &out); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer secret-token" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer secret-token")
	}
}

func TestEnvelopeDecode(t *testing.T) {
	type Item struct{ ID int }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":1}],"pagination":{"total":1,"page":1,"per_page":10}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	env, err := httpclient.GetEnvelope[[]Item](context.Background(), c, "/items")
	if err != nil {
		t.Fatal(err)
	}
	if len(env.Data) != 1 || env.Data[0].ID != 1 {
		t.Errorf("Data = %+v", env.Data)
	}
	if env.Pagination == nil || env.Pagination.Total != 1 {
		t.Errorf("Pagination = %+v", env.Pagination)
	}
}

func TestRetryOn503(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	var out any
	if err := c.Get(context.Background(), "/", &out); err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", atomic.LoadInt32(&attempts))
	}
}

func TestNoRetryOn404(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"not found"}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	var out any
	err := c.Get(context.Background(), "/", &out)
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeNotFound {
		t.Errorf("code = %d, want CodeNotFound", ce.Code)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1 (no retry on 404)", atomic.LoadInt32(&attempts))
	}
}

func TestContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	c := httpclient.New(srv.URL, "tok")
	var out any
	err := c.Get(ctx, "/", &out)
	if err == nil {
		t.Fatal("expected error from cancelled context")
	}
}

func TestPostEnvelope(t *testing.T) {
	type Item struct{ ID int }
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decoding body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"id":42}}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	env, err := httpclient.PostEnvelope[Item](context.Background(), c, "/items", map[string]string{"name": "test"})
	if err != nil {
		t.Fatal(err)
	}
	if env.Data.ID != 42 {
		t.Errorf("Data.ID = %d, want 42", env.Data.ID)
	}
}
