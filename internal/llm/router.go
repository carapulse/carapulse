package llm

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"
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

// injectionPatterns matches common prompt injection attempts in external data.
var injectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?above\s+instructions`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?previous`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?previous`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+a`),
	regexp.MustCompile(`(?i)new\s+instructions?\s*:`),
	regexp.MustCompile(`(?i)system\s*:\s*you`),
	regexp.MustCompile(`(?i)<<\s*SYS\s*>>`),
	regexp.MustCompile(`(?i)\[INST\]`),
	regexp.MustCompile(`(?i)\[/INST\]`),
	regexp.MustCompile(`(?i)<\|im_start\|>`),
	regexp.MustCompile(`(?i)<\|im_end\|>`),
}

// SanitizePromptInput cleans external data before including it in LLM prompts.
// It strips control characters and common prompt injection patterns.
func SanitizePromptInput(input string) string {
	// Strip control characters (except newline, tab, carriage return).
	cleaned := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' || r == '\r' {
			return r
		}
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, input)

	// Replace prompt injection patterns.
	for _, re := range injectionPatterns {
		cleaned = re.ReplaceAllString(cleaned, "[FILTERED]")
	}

	return cleaned
}
