package tools

import "testing"

func TestProxyEnv(t *testing.T) {
	env, closeFn, err := proxyEnv([]string{"example.com"}, "docker")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer closeFn()
	if env["HTTP_PROXY"] == "" || env["HTTPS_PROXY"] == "" {
		t.Fatalf("missing proxy env")
	}
}
