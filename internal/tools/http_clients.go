package tools

type HTTPClients struct {
	Prometheus   *APIClient
	Alertmanager *APIClient
	Thanos       *APIClient
	Grafana      *APIClient
	Tempo        *APIClient
	Linear       *APIClient
	PagerDuty    *APIClient
	Vault        *APIClient
	Boundary     *APIClient
	ArgoCD       *APIClient
	AWS          *APIClient
}
