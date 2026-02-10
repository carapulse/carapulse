package tools

import (
	"net/http"

	"carapulse/internal/auth"
)

func ParseBearer(r *http.Request) (string, error) {
	return auth.ParseBearer(r)
}
