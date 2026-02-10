package tools

import (
	"errors"
	"strings"
)

// shellMetaChars are characters that could enable command injection when passed
// through to a shell.  The tool router builds argument arrays (no shell
// involved), but defense-in-depth blocks these at the validation boundary.
const shellMetaChars = ";|&$`!(){}[]<>\\\"'\n\r"

var (
	errDangerousArg = errors.New("argument contains shell metacharacters")
	errEmptyArg     = errors.New("empty argument in command")
)

// ValidateToolArgs checks every argument in a CLI command for shell
// metacharacters.  It is called after buildCmd and before sandbox.Run to
// prevent injection regardless of how the command is eventually executed.
func ValidateToolArgs(cmd []string) error {
	if len(cmd) == 0 {
		return nil
	}
	// cmd[0] is the tool binary name; start checking from arguments.
	for i := 1; i < len(cmd); i++ {
		if err := validateArg(cmd[i]); err != nil {
			return err
		}
	}
	return nil
}

func validateArg(arg string) error {
	if arg == "" {
		return errEmptyArg
	}
	if strings.ContainsAny(arg, shellMetaChars) {
		return errDangerousArg
	}
	return nil
}

// ValidateToolName verifies that the requested tool name matches a
// tool in the registry.  Returns the matched tool or an error.
func ValidateToolName(name string) (*Tool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errToolRequired
	}
	tool := findTool(name)
	if tool == nil {
		return nil, errors.New("unknown tool: " + name)
	}
	return tool, nil
}
