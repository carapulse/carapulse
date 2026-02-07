package tools

import (
	"errors"
	"net/http"
	"strings"
)

func ParseBearer(r *http.Request) (string, error) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return "", errors.New("authorization required")
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return "", errors.New("invalid authorization")
	}
	return strings.TrimSpace(parts[1]), nil
}
