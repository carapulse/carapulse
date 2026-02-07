package web

import (
	"errors"
	"net/http"
	"strings"
)

func ParseBearer(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return "", errors.New("missing auth")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return "", errors.New("invalid auth")
	}
	return parts[1], nil
}
