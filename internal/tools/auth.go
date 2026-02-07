package tools

type AuthConfig struct {
	Issuer   string
	Audience string
	JWKSURL  string
	Token    string
}
