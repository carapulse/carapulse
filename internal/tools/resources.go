package tools

type Resource struct {
	Name string `json:"name"`
	Type string `json:"type"`
	URI  string `json:"uri"`
}

type Prompt struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

var workspaceDir string

func SetWorkspaceDir(dir string) {
	workspaceDir = dir
}

func ListResources() []Resource {
	if workspaceDir == "" {
		return []Resource{}
	}
	var out []Resource
	out = append(out, loadRunbookResources()...)
	out = append(out, loadPlaybookResources()...)
	out = append(out, loadWorkflowResources()...)
	return out
}

func ListPrompts() []Prompt {
	if workspaceDir == "" {
		return []Prompt{}
	}
	return loadPrompts()
}
