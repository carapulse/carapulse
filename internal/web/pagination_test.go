package web

import (
	"net/http/httptest"
	"testing"
)

func TestParsePaginationDefaults(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	limit, offset := parsePagination(req)
	if limit != 50 {
		t.Fatalf("expected default limit 50, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected default offset 0, got %d", offset)
	}
}

func TestParsePaginationCustom(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=25&offset=10", nil)
	limit, offset := parsePagination(req)
	if limit != 25 {
		t.Fatalf("expected limit 25, got %d", limit)
	}
	if offset != 10 {
		t.Fatalf("expected offset 10, got %d", offset)
	}
}

func TestParsePaginationMaxLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=500", nil)
	limit, _ := parsePagination(req)
	if limit != 200 {
		t.Fatalf("expected max limit 200, got %d", limit)
	}
}

func TestParsePaginationNegativeValues(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=-5&offset=-3", nil)
	limit, offset := parsePagination(req)
	if limit != 50 {
		t.Fatalf("expected default limit 50 for negative, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected default offset 0 for negative, got %d", offset)
	}
}

func TestParsePaginationNonNumeric(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=abc&offset=xyz", nil)
	limit, offset := parsePagination(req)
	if limit != 50 {
		t.Fatalf("expected default limit 50 for non-numeric, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected default offset 0 for non-numeric, got %d", offset)
	}
}

func TestParsePaginationZeroLimit(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=0", nil)
	limit, _ := parsePagination(req)
	if limit != 50 {
		t.Fatalf("expected default limit 50 for zero, got %d", limit)
	}
}

func TestParsePaginationZeroOffset(t *testing.T) {
	req := httptest.NewRequest("GET", "/?offset=0", nil)
	_, offset := parsePagination(req)
	if offset != 0 {
		t.Fatalf("expected offset 0, got %d", offset)
	}
}

func TestParsePaginationLimitExactly200(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=200", nil)
	limit, _ := parsePagination(req)
	if limit != 200 {
		t.Fatalf("expected limit 200, got %d", limit)
	}
}
