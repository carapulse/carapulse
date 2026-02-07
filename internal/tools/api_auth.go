package tools

import "net/http"

type AuthHeaders struct {
	BearerToken string
	BasicUser   string
	BasicPass   string
	Extra       map[string]string
}

func ApplyAuth(req *http.Request, auth AuthHeaders) {
	if auth.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+auth.BearerToken)
	}
	if auth.BasicUser != "" {
		req.SetBasicAuth(auth.BasicUser, auth.BasicPass)
	}
	for k, v := range auth.Extra {
		req.Header.Set(k, v)
	}
}
