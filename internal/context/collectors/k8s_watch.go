package collectors

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/tools"
)

var defaultK8sResources = []string{"deployments", "statefulsets", "daemonsets", "services", "ingresses", "pods", "events", "nodes"}

func DefaultK8sResources() []string {
	out := make([]string, 0, len(defaultK8sResources))
	out = append(out, defaultK8sResources...)
	return out
}

type K8sPoller struct {
	Base
	Resources []string
	Namespace string
}

func (p *K8sPoller) Snapshot(ctx context.Context) (ctxmodel.Snapshot, error) {
	if p.Router == nil {
		return ctxmodel.Snapshot{}, errors.New("router required")
	}
	resources := p.Resources
	if len(resources) == 0 {
		resources = defaultK8sResources
	}
	snap := ctxmodel.Snapshot{}
	for _, res := range resources {
		input := map[string]any{"resource": res}
		if strings.TrimSpace(p.Namespace) != "" {
			input["namespace"] = p.Namespace
		}
		resp, err := p.Router.Execute(ctx, tools.ExecuteRequest{Tool: "kubectl", Action: "get", Input: input, Context: p.Context})
		if err != nil {
			return ctxmodel.Snapshot{}, err
		}
		nodes, edges, _ := snapshotFromK8sList(resp.Output, p.withLabels(nil))
		snap.Nodes = append(snap.Nodes, nodes...)
		snap.Edges = append(snap.Edges, edges...)
	}
	return snap, nil
}

type K8sWatcher struct {
	Base
	Resources            []string
	Namespace            string
	Selector             string
	Timeout              time.Duration
	PollInterval         time.Duration
	LastResourceVersions map[string]string
	SendInitialEvents    bool
	AllowBookmarks       bool
}

func (w *K8sWatcher) Watch(ctx context.Context, out chan<- ctxmodel.Snapshot) error {
	if w.Router == nil {
		return errors.New("router required")
	}
	resources := w.Resources
	if len(resources) == 0 {
		resources = defaultK8sResources
	}
	if w.Timeout <= 0 {
		w.Timeout = 30 * time.Second
	}
	if w.PollInterval <= 0 {
		w.PollInterval = 2 * time.Second
	}
	if w.LastResourceVersions == nil {
		w.LastResourceVersions = map[string]string{}
	}
	for {
		for _, res := range resources {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
			input := map[string]any{
				"resource":        res,
				"timeout_seconds": int(w.Timeout.Seconds()),
			}
			if rv := strings.TrimSpace(w.LastResourceVersions[res]); rv != "" {
				input["resource_version"] = rv
			}
			if strings.TrimSpace(w.Namespace) != "" {
				input["namespace"] = w.Namespace
			}
			if strings.TrimSpace(w.Selector) != "" {
				input["selector"] = w.Selector
			}
			if w.SendInitialEvents {
				input["send_initial_events"] = true
			}
			if w.AllowBookmarks {
				input["allow_bookmarks"] = true
			}
			resp, err := w.Router.Execute(ctx, tools.ExecuteRequest{Tool: "kubectl", Action: "watch", Input: input, Context: w.Context})
			if err != nil {
				continue
			}
			snap, rv := snapshotFromK8sWatch(resp.Output, w.withLabels(nil))
			if rv != "" {
				w.LastResourceVersions[res] = rv
			}
			if len(snap.Nodes) > 0 || len(snap.Edges) > 0 {
				out <- snap
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(w.PollInterval):
		}
	}
}

func snapshotFromK8sList(payload []byte, labels map[string]string) ([]ctxmodel.Node, []ctxmodel.Edge, string) {
	if len(payload) == 0 {
		return nil, nil, ""
	}
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, nil, ""
	}
	rv := resourceVersionFromMeta(raw["metadata"])
	items, _ := raw["items"].([]any)
	var nodes []ctxmodel.Node
	var edges []ctxmodel.Edge
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		resNodes, resEdges, _ := snapshotFromK8sObject(obj, labels)
		nodes = append(nodes, resNodes...)
		edges = append(edges, resEdges...)
	}
	return nodes, edges, rv
}

func snapshotFromK8sWatch(payload []byte, labels map[string]string) (ctxmodel.Snapshot, string) {
	lines := strings.Split(strings.TrimSpace(string(payload)), "\n")
	var snap ctxmodel.Snapshot
	lastRV := ""
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		obj, ok := event["object"].(map[string]any)
		if !ok {
			continue
		}
		nodes, edges, rv := snapshotFromK8sObject(obj, labels)
		snap.Nodes = append(snap.Nodes, nodes...)
		snap.Edges = append(snap.Edges, edges...)
		if rv != "" {
			lastRV = rv
		}
	}
	return snap, lastRV
}

func snapshotFromK8sObject(obj map[string]any, labels map[string]string) ([]ctxmodel.Node, []ctxmodel.Edge, string) {
	meta, _ := obj["metadata"].(map[string]any)
	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	kind, _ := obj["kind"].(string)
	if name == "" || kind == "" {
		return nil, nil, ""
	}
	resourceID := nodeID("k8s", strings.ToLower(kind), namespace, name)
	resourceLabels := map[string]string{"namespace": namespace, "kind": kind}
	for k, v := range labels {
		resourceLabels[k] = v
	}
	k8sNode := node("k8s."+strings.ToLower(kind), resourceID, name, resourceLabels)
	var nodes []ctxmodel.Node
	var edges []ctxmodel.Edge
	nodes = append(nodes, k8sNode)
	if namespace != "" {
		nsID := nodeID("k8s", "namespace", namespace)
		nodes = append(nodes, node("k8s.namespace", nsID, namespace, labels))
		edges = append(edges, edge(nodeID("edge", nsID, resourceID), nsID, resourceID, "contains"))
	}
	return nodes, edges, resourceVersionFromMeta(meta)
}

func resourceVersionFromMeta(meta any) string {
	if m, ok := meta.(map[string]any); ok {
		if v, ok := m["resourceVersion"].(string); ok {
			return v
		}
	}
	return ""
}
