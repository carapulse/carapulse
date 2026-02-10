package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	ctxmodel "carapulse/internal/context"
	"carapulse/internal/policy"
)

func TestHandleContextSnapshotByIDGet(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
}

func TestHandleContextSnapshotByIDMethodNotAllowed(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodPost, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDNoTenant(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDNoDB(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: nil, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDPolicyDeny(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: denyChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDMissingID(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestParseSnapshotPayload(t *testing.T) {
	payload := []byte(`{"snapshot_id":"snap_1","labels":{"env":"prod"},"nodes":[{"node_id":"n1"}],"edges":[{"edge_id":"e1"}]}`)
	item, err := parseSnapshotPayload(payload)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if item.SnapshotID != "snap_1" {
		t.Fatalf("snapshot_id: %s", item.SnapshotID)
	}
	if len(item.Nodes) != 1 || item.Nodes[0].NodeID != "n1" {
		t.Fatalf("nodes: %#v", item.Nodes)
	}
	if len(item.Edges) != 1 || item.Edges[0].EdgeID != "e1" {
		t.Fatalf("edges: %#v", item.Edges)
	}
}

func TestParseSnapshotPayloadInvalid(t *testing.T) {
	if _, err := parseSnapshotPayload([]byte("{")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestDiffSnapshots(t *testing.T) {
	current := snapshotPayload{
		Nodes: []ctxmodel.Node{{NodeID: "n1"}, {NodeID: "n2"}},
		Edges: []ctxmodel.Edge{{EdgeID: "e1"}, {EdgeID: "e2"}},
	}
	base := snapshotPayload{
		Nodes: []ctxmodel.Node{{NodeID: "n1"}, {NodeID: "n3"}},
		Edges: []ctxmodel.Edge{{EdgeID: "e1"}, {EdgeID: "e3"}},
	}
	result := diffSnapshots(current, base)
	addedNodes := result["added_nodes"].([]ctxmodel.Node)
	removedNodes := result["removed_nodes"].([]ctxmodel.Node)
	addedEdges := result["added_edges"].([]ctxmodel.Edge)
	removedEdges := result["removed_edges"].([]ctxmodel.Edge)

	if len(addedNodes) != 1 || addedNodes[0].NodeID != "n2" {
		t.Fatalf("added nodes: %#v", addedNodes)
	}
	if len(removedNodes) != 1 || removedNodes[0].NodeID != "n3" {
		t.Fatalf("removed nodes: %#v", removedNodes)
	}
	if len(addedEdges) != 1 || addedEdges[0].EdgeID != "e2" {
		t.Fatalf("added edges: %#v", addedEdges)
	}
	if len(removedEdges) != 1 || removedEdges[0].EdgeID != "e3" {
		t.Fatalf("removed edges: %#v", removedEdges)
	}
}

func TestHandleContextSnapshotByIDDiff(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1/diff?base=snap_2", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d body: %s", w.Code, w.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result["snapshot_id"] != "snap_1" || result["base_snapshot_id"] != "snap_2" {
		t.Fatalf("diff result: %#v", result)
	}
}

func TestHandleContextSnapshotByIDDiffMissingBase(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1/diff", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleContextSnapshotByIDInvalidPath(t *testing.T) {
	enableDevMode(t)
	srv := &Server{Mux: http.NewServeMux(), DB: missingExecDB{}, Policy: &policy.Evaluator{Checker: allowChecker{}}, FailOpenReads: true}
	req := httptest.NewRequest(http.MethodGet, "/v1/context/snapshots/snap_1/invalid/extra", nil)
	req.Header.Set("Authorization", "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb")
	req.Header.Set("X-Tenant-Id", "t")
	w := httptest.NewRecorder()
	AuthMiddleware(http.HandlerFunc(srv.handleContextSnapshotByID)).ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}
