package auth

import (
	"errors"
	"net/http"
	"strings"
)

// ParseBearer extracts a bearer token from the Authorization header. It trims
// whitespace and uses case-insensitive comparison for the "Bearer" prefix.
func ParseBearer(r *http.Request) (string, error) {
	hdr := strings.TrimSpace(r.Header.Get("Authorization"))
	if hdr == "" {
		return "", errors.New("authorization required")
	}
	parts := strings.SplitN(hdr, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("invalid authorization")
	}
	return strings.TrimSpace(parts[1]), nil
}
