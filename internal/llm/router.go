package llm

import (
	"net/http"
	"regexp"
	"strings"
)

type Planner interface {
	Plan(intent string, context any, evidence any) (string, error)
}

type Router struct {
	Provider       string
	Model          string
	APIBase        string
	APIKey         string
	MaxTokens      int
	HTTPClient     *http.Client
	AuthProfile    string
	AuthPath       string
	RedactPatterns []string
}

func Redact(input string, patterns []string) string {
	out := input
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			continue
		}
		out = re.ReplaceAllString(out, "[REDACTED]")
	}
	return strings.TrimSpace(out)
}
