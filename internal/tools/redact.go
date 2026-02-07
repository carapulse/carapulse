package tools

import "regexp"

type Redactor struct {
	patterns []*regexp.Regexp
}

func DefaultRedactPatterns() []string {
	return []string{
		`(?i)token=\\w+`,
		`(?i)secret=\\w+`,
		`(?i)x-api-key:\\s*\\S+`,
	}
}

func NewRedactor(patterns []string) *Redactor {
	if len(patterns) == 0 {
		return nil
	}
	var compiled []*regexp.Regexp
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if re, err := regexp.Compile(pattern); err == nil {
			compiled = append(compiled, re)
		}
	}
	if len(compiled) == 0 {
		return nil
	}
	return &Redactor{patterns: compiled}
}

func (r *Redactor) Redact(input []byte) []byte {
	if r == nil || len(input) == 0 {
		return input
	}
	out := string(input)
	for _, re := range r.patterns {
		out = re.ReplaceAllString(out, "***")
	}
	return []byte(out)
}

func (r *Redactor) RedactString(input string) string {
	if r == nil || input == "" {
		return input
	}
	out := input
	for _, re := range r.patterns {
		out = re.ReplaceAllString(out, "***")
	}
	return out
}
