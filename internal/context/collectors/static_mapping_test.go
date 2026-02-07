package collectors

import "testing"

func TestStaticMappingPollerSnapshot(t *testing.T) {
	poller := &StaticMappingPoller{
		Mappings: []ServiceMapping{
			{
				Service:     "api",
				Environment: "prod",
				ClusterID:   "c1",
				Namespace:   "ns",
				PromQL:      []string{"up"},
				TraceQL:     []string{"{ }"},
				Dashboards:  []string{"dash1"},
			},
		},
	}
	snap, err := poller.Snapshot(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
	if len(snap.Edges) == 0 {
		t.Fatalf("expected edges")
	}
}
