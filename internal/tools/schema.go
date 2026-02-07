package tools

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

//go:embed schemas/*.json
var schemaFS embed.FS

var schemaCache sync.Map

type schemaDoc struct {
	Actions map[string]json.RawMessage `json:"actions"`
}

func validateSchema(tool, action string, input any) error {
	actions, err := loadToolSchema(tool)
	if err != nil {
		return nil
	}
	raw, ok := actions[action]
	if !ok {
		raw, ok = actions["*"]
	}
	if !ok {
		return nil
	}
	var schema any
	if err := json.Unmarshal(raw, &schema); err != nil {
		return err
	}
	value := normalizeSchemaInput(input)
	result, err := gojsonschema.Validate(gojsonschema.NewGoLoader(schema), gojsonschema.NewGoLoader(value))
	if err != nil {
		return err
	}
	if result.Valid() {
		return nil
	}
	if len(result.Errors()) == 0 {
		return errors.New("schema validation failed")
	}
	return fmt.Errorf("schema validation failed: %s", result.Errors()[0].String())
}

func normalizeSchemaInput(input any) any {
	if input == nil {
		return map[string]any{}
	}
	switch v := input.(type) {
	case []byte:
		var out any
		if err := json.Unmarshal(v, &out); err == nil {
			return out
		}
	case json.RawMessage:
		var out any
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			return out
		}
	case string:
		if strings.TrimSpace(v) == "" {
			return map[string]any{}
		}
		var out any
		if err := json.Unmarshal([]byte(v), &out); err == nil {
			return out
		}
	}
	return input
}

func loadToolSchema(tool string) (map[string]json.RawMessage, error) {
	if val, ok := schemaCache.Load(tool); ok {
		return val.(map[string]json.RawMessage), nil
	}
	data, err := schemaFS.ReadFile("schemas/" + tool + ".json")
	if err != nil {
		return nil, err
	}
	var doc schemaDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Actions) == 0 {
		return nil, errors.New("no actions")
	}
	schemaCache.Store(tool, doc.Actions)
	return doc.Actions, nil
}
