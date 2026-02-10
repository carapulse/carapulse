package auth

// AuthConfig holds authentication configuration used by both the gateway and
// tool-router services. DevMode skips JWT signature verification when no JWKS
// URL is configured. Token provides a static service-to-service bearer token.
type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
	DevMode  bool
	Token    string
}
