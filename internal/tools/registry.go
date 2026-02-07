package tools

type Tool struct {
	Name        string
	CLI         string
	SupportsAPI bool
}

var Registry = []Tool{
	{Name: "aws", CLI: "aws", SupportsAPI: true},
	{Name: "vault", CLI: "vault", SupportsAPI: true},
	{Name: "kubectl", CLI: "kubectl", SupportsAPI: true},
	{Name: "helm", CLI: "helm", SupportsAPI: true},
	{Name: "boundary", CLI: "boundary", SupportsAPI: true},
	{Name: "argocd", CLI: "argocd", SupportsAPI: true},
	{Name: "git", CLI: "git", SupportsAPI: false},
	{Name: "prometheus", CLI: "", SupportsAPI: true},
	{Name: "alertmanager", CLI: "", SupportsAPI: true},
	{Name: "thanos", CLI: "", SupportsAPI: true},
	{Name: "grafana", CLI: "", SupportsAPI: true},
	{Name: "tempo", CLI: "", SupportsAPI: true},
	{Name: "github", CLI: "gh", SupportsAPI: true},
	{Name: "gitlab", CLI: "glab", SupportsAPI: true},
	{Name: "linear", CLI: "", SupportsAPI: true},
	{Name: "pagerduty", CLI: "", SupportsAPI: true},
}
