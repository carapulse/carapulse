package secrets

import (
	"bufio"
	"errors"
	"os"
	"strings"
)

func LoadTemplateEnv(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	env := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		if key == "" {
			continue
		}
		env[key] = val
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(env) == 0 {
		return nil, errors.New("no env entries")
	}
	return env, nil
}
