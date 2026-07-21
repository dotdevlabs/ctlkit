package httpclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dotdevlabs/ctlkit/pkg/clierror"
	"github.com/dotdevlabs/ctlkit/pkg/httpclient"
)

func TestPatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Errorf("method = %q, want PATCH", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	var out any
	if err := c.Patch(context.Background(), "/", map[string]string{"key": "val"}, &out); err != nil {
		t.Fatal(err)
	}
}

func TestDelete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	if err := c.Delete(context.Background(), "/item/1"); err != nil {
		t.Fatal(err)
	}
}

func TestUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"unauthorized"}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "bad-tok")
	err := c.Delete(context.Background(), "/")
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeUnauthorized {
		t.Errorf("code = %d, want CodeUnauthorized", ce.Code)
	}
}

func TestForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	err := c.Get(context.Background(), "/", nil)
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeForbidden {
		t.Errorf("code = %d, want CodeForbidden", ce.Code)
	}
}

func TestConflict(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	err := c.Post(context.Background(), "/", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeConflict {
		t.Errorf("code = %d, want CodeConflict", ce.Code)
	}
}

func TestBadRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"bad input"}`))
	}))
	defer srv.Close()

	c := httpclient.New(srv.URL, "tok")
	err := c.Post(context.Background(), "/", map[string]string{"x": "y"}, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	ce, ok := err.(*clierror.CLIError)
	if !ok {
		t.Fatalf("expected CLIError, got %T", err)
	}
	if ce.Code != clierror.CodeBadRequest {
		t.Errorf("code = %d, want CodeBadRequest", ce.Code)
	}
}
