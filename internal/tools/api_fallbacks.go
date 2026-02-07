package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type APIClient struct {
	BaseURL   string
	Client    *http.Client
	Auth      AuthHeaders
	Allowlist []string
	TokenFile string
	MaxOutputBytes int
}

func (c *APIClient) Do(ctx context.Context, method, path string, body any) ([]byte, error) {
	if c.Client == nil {
		c.Client = &http.Client{}
	}
	if len(c.Allowlist) > 0 {
		if !allowHost(c.BaseURL, c.Allowlist) {
			return nil, errors.New("egress denied")
		}
	}
	var buf *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewReader(data)
	} else {
		buf = bytes.NewReader(nil)
	}
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", c.BaseURL, path), buf)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	auth := c.Auth
	if auth.BearerToken == "" && strings.TrimSpace(c.TokenFile) != "" {
		if token, err := readTokenFile(c.TokenFile); err == nil && token != "" {
			auth.BearerToken = token
		}
	}
	ApplyAuth(req, auth)
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	reader := io.Reader(resp.Body)
	if c.MaxOutputBytes > 0 {
		reader = io.LimitReader(resp.Body, int64(c.MaxOutputBytes)+1)
	}
	data, err := ioReadAll(reader)
	if err != nil {
		return nil, err
	}
	if c.MaxOutputBytes > 0 && len(data) > c.MaxOutputBytes {
		trimmed := data[:c.MaxOutputBytes]
		return append(trimmed, []byte("...(truncated)")...), nil
	}
	return data, nil
}

func ioReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

var readFile = os.ReadFile

func readTokenFile(path string) (string, error) {
	data, err := readFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func allowHost(baseURL string, allowlist []string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "" {
		return false
	}
	for _, allowed := range allowlist {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}
		if strings.EqualFold(host, allowed) {
			return true
		}
		if strings.HasPrefix(allowed, "*.") && strings.HasSuffix(host, strings.TrimPrefix(allowed, "*")) {
			return true
		}
	}
	return false
}
