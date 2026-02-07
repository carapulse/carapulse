package collectors

import (
	"context"
	"encoding/json"
	"testing"

	"carapulse/internal/tools"
)

func TestAWSPollerSnapshot(t *testing.T) {
	payload := map[string]any{
		"ResourceTagMappingList": []any{
			map[string]any{"ResourceARN": "arn:aws:s3:us-east-1:123456789012:bucket/my-bucket"},
		},
	}
	data, _ := json.Marshal(payload)
	runner := &fakeRunner{resp: tools.ExecuteResponse{Output: data}}
	poller := &AWSPoller{Base: Base{Router: runner}}
	snap, err := poller.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(snap.Nodes) == 0 {
		t.Fatalf("expected nodes")
	}
}
