package tools

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestAPIClientDo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	client := &APIClient{BaseURL: srv.URL}
	resp, err := client.Do(context.Background(), "GET", "/", nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(resp) != "ok" {
		t.Fatalf("resp: %s", string(resp))
	}
}

func TestAPIClientTokenFile(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/token"
	if err := os.WriteFile(path, []byte("tok"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer tok" {
			t.Fatalf("auth: %s", got)
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()
	client := &APIClient{BaseURL: srv.URL, TokenFile: path}
	if _, err := client.Do(context.Background(), "GET", "/", nil); err != nil {
		t.Fatalf("err: %v", err)
	}
}

func TestAPIClientDoBadBody(t *testing.T) {
	client := &APIClient{BaseURL: "http://example.com"}
	_, err := client.Do(context.Background(), "POST", "/", make(chan int))
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIClientDoBadURL(t *testing.T) {
	client := &APIClient{BaseURL: "http://%"}
	if _, err := client.Do(context.Background(), "GET", "/", nil); err == nil {
		t.Fatalf("expected error")
	}
}

type errRoundTripper struct{ err error }

func (e errRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

type errBody struct{}

func (e errBody) Read(p []byte) (int, error) { return 0, io.ErrUnexpectedEOF }
func (e errBody) Close() error               { return nil }

type bodyRoundTripper struct{}

func (b bodyRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusOK, Body: errBody{}}, nil
}

func TestAPIClientDoRequestError(t *testing.T) {
	client := &APIClient{
		BaseURL: "http://example.com",
		Client:  &http.Client{Transport: errRoundTripper{err: errors.New("boom")}},
	}
	if _, err := client.Do(context.Background(), "GET", "/", nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAPIClientDoReadError(t *testing.T) {
	client := &APIClient{
		BaseURL: "http://example.com",
		Client:  &http.Client{Transport: bodyRoundTripper{}},
	}
	if _, err := client.Do(context.Background(), "GET", "/", nil); err == nil {
		t.Fatalf("expected error")
	}
}
