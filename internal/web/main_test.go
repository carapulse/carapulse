package web

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	// Default to DevMode for tests so existing tests that use fake tokens
	// without JWKS continue to work. Tests that specifically validate the
	// SEC-02 fix override this per-test.
	SetAuthConfig(AuthConfig{DevMode: true})
	os.Exit(m.Run())
}
