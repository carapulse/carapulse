package collectors

import (
	"context"
	"errors"
	"strings"

	ctxmodel "carapulse/internal/context"
)

type ServiceMapping struct {
	Service     string
	Environment string
	ClusterID   string
	Namespace   string
	PromQL      []string
	TraceQL     []string
	Dashboards  []string
}

type StaticMappingPoller struct {
	Base
	Mappings []ServiceMapping
}

func (p *StaticMappingPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	_ = ctx
	if len(p.Mappings) == 0 {
		return ctxmodel.Snapshot{}, errors.New("mappings required")
	}
	var snap ctxmodel.Snapshot
	for _, mapping := range p.Mappings {
		service := strings.TrimSpace(mapping.Service)
		if service == "" {
			continue
		}
		svcID := nodeID("service", service)
		labels := p.withLabels(map[string]string{
			"environment": mapping.Environment,
			"cluster_id":  mapping.ClusterID,
			"namespace":   mapping.Namespace,
		})
		snap.Nodes = append(snap.Nodes, node("service", svcID, service, labels))
		if env := strings.TrimSpace(mapping.Environment); env != "" {
			envID := nodeID("env", env)
			snap.Nodes = append(snap.Nodes, node("env", envID, env, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, envID), svcID, envID, "runs_in"))
		}
		if cluster := strings.TrimSpace(mapping.ClusterID); cluster != "" {
			clusterID := nodeID("cluster", cluster)
			snap.Nodes = append(snap.Nodes, node("cluster", clusterID, cluster, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, clusterID), svcID, clusterID, "runs_on"))
		}
		if ns := strings.TrimSpace(mapping.Namespace); ns != "" {
			nsID := nodeID("k8s", "namespace", ns)
			snap.Nodes = append(snap.Nodes, node("k8s.namespace", nsID, ns, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, nsID), svcID, nsID, "owns"))
		}
		for _, query := range mapping.PromQL {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			queryID := nodeID("prometheus", "query", hash(query))
			snap.Nodes = append(snap.Nodes, node("prometheus.query", queryID, query, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, queryID), svcID, queryID, "monitors"))
		}
		for _, query := range mapping.TraceQL {
			query = strings.TrimSpace(query)
			if query == "" {
				continue
			}
			queryID := nodeID("tempo", "query", hashTempo(query))
			snap.Nodes = append(snap.Nodes, node("tempo.query", queryID, query, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, queryID), svcID, queryID, "traces"))
		}
		for _, dash := range mapping.Dashboards {
			dash = strings.TrimSpace(dash)
			if dash == "" {
				continue
			}
			dashID := nodeID("grafana", "dashboard", dash)
			snap.Nodes = append(snap.Nodes, node("grafana.dashboard", dashID, dash, labels))
			snap.Edges = append(snap.Edges, edge(nodeID("edge", svcID, dashID), svcID, dashID, "dashboards"))
		}
	}
	return snap, nil
}
