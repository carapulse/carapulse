package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestJsonbBuildObjectQuoting scans all .go source files in internal/db/ and
// fails if any jsonb_build_object call uses double-quoted keys. PostgreSQL
// interprets "name" as a column identifier, not a string literal — the correct
// syntax is 'name'. This is a permanent regression guard for the SQL quoting
// bug that caused every LIST endpoint to return 500 against real Postgres.
func TestJsonbBuildObjectQuoting(t *testing.T) {
	// Match jsonb_build_object( ... "some_key" ... ) — double-quoted keys
	// inside a jsonb_build_object call. We look for lines inside a
	// jsonb_build_object block that have a double-quoted identifier followed
	// by a comma and a column reference, which is the broken pattern.
	doubleQuotedKey := regexp.MustCompile(`"\w+"\s*,\s*\w+`)

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(data)

		// Find all jsonb_build_object blocks and check for double-quoted keys.
		// We split on jsonb_build_object and check each subsequent chunk up to
		// the closing paren.
		parts := strings.Split(content, "jsonb_build_object(")
		if len(parts) <= 1 {
			continue
		}
		for i, part := range parts[1:] {
			// Find the closing paren of this jsonb_build_object call.
			// We need to handle nested parens.
			depth := 1
			end := -1
			for j := 0; j < len(part); j++ {
				if part[j] == '(' {
					depth++
				} else if part[j] == ')' {
					depth--
					if depth == 0 {
						end = j
						break
					}
				}
			}
			if end == -1 {
				continue
			}
			block := part[:end]

			if doubleQuotedKey.MatchString(block) {
				// Find the approximate line number.
				prefix := strings.Join(parts[:i+1], "jsonb_build_object(")
				lineNum := strings.Count(prefix, "\n") + 1
				t.Errorf("%s:%d: jsonb_build_object uses double-quoted key (Postgres treats these as column identifiers, not string literals):\n%s",
					file, lineNum, trimBlock(block))
			}
		}
	}
}

func trimBlock(s string) string {
	lines := strings.Split(s, "\n")
	if len(lines) > 15 {
		lines = lines[:15]
		lines = append(lines, "\t...")
	}
	return strings.Join(lines, "\n")
}
