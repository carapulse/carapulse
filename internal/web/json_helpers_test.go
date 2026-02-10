package web

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONHappyPath(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"key": "value"})
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: %s", ct)
	}
	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["key"] != "value" {
		t.Fatalf("got: %v", got)
	}
}

func TestWriteJSONCustomStatus(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusCreated, map[string]string{"id": "123"})
	if w.Code != http.StatusCreated {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestWriteJSONMarshalError(t *testing.T) {
	w := httptest.NewRecorder()
	// math.NaN() cannot be marshaled to JSON
	writeJSON(w, http.StatusOK, math.NaN())
	// Should still set status and content-type, but body will be empty or partial
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("content-type: %s", ct)
	}
}
