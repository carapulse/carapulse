package db

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	ctxmodel "carapulse/internal/context"
)

func TestUpsertContextNodeOK(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	node := ctxmodel.Node{
		NodeID:    "node_1",
		Kind:      "service",
		Name:      "svc",
		Labels:    map[string]string{"env": "prod"},
		OwnerTeam: "platform",
	}
	if err := d.UpsertContextNode(context.Background(), node); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "context_nodes") {
		t.Fatalf("missing query")
	}
	if got := len(conn.lastExecArgs); got != 5 {
		t.Fatalf("args: %d", got)
	}
	if conn.lastExecArgs[0] != "node_1" || conn.lastExecArgs[1] != "service" || conn.lastExecArgs[2] != "svc" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
	if conn.lastExecArgs[4] != "platform" {
		t.Fatalf("owner_team: %#v", conn.lastExecArgs[4])
	}
	if _, ok := conn.lastExecArgs[3].([]byte); !ok {
		t.Fatalf("labels type: %#v", conn.lastExecArgs[3])
	}
}

func TestUpsertContextNodeNilLabels(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	node := ctxmodel.Node{
		NodeID: "node_2",
		Kind:   "service",
		Name:   "svc2",
	}
	if err := d.UpsertContextNode(context.Background(), node); err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(conn.lastExecArgs[3].([]byte)) != "null" {
		t.Fatalf("labels: %s", string(conn.lastExecArgs[3].([]byte)))
	}
	if conn.lastExecArgs[4] != nil {
		t.Fatalf("owner_team: %#v", conn.lastExecArgs[4])
	}
}

func TestUpsertContextNodeInvalid(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpsertContextNode(context.Background(), ctxmodel.Node{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertContextNodeExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	node := ctxmodel.Node{NodeID: "node", Kind: "service", Name: "svc"}
	if err := d.UpsertContextNode(context.Background(), node); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertContextEdgeOK(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	edge := ctxmodel.Edge{
		EdgeID:     "edge_1",
		FromNodeID: "node_1",
		ToNodeID:   "node_2",
		Relation:   "depends_on",
	}
	if err := d.UpsertContextEdge(context.Background(), edge); err != nil {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(conn.lastExecQuery, "context_edges") {
		t.Fatalf("missing query")
	}
	if got := len(conn.lastExecArgs); got != 4 {
		t.Fatalf("args: %d", got)
	}
	if conn.lastExecArgs[0] != "edge_1" || conn.lastExecArgs[1] != "node_1" || conn.lastExecArgs[2] != "node_2" {
		t.Fatalf("args: %#v", conn.lastExecArgs)
	}
}

func TestUpsertContextEdgeGeneratedID(t *testing.T) {
	conn := &fakeConn{}
	d := &DB{conn: conn}
	edge := ctxmodel.Edge{
		FromNodeID: "node_1",
		ToNodeID:   "node_2",
		Relation:   "calls",
	}
	if err := d.UpsertContextEdge(context.Background(), edge); err != nil {
		t.Fatalf("err: %v", err)
	}
	edgeID, ok := conn.lastExecArgs[0].(string)
	if !ok || !strings.HasPrefix(edgeID, "edge_") {
		t.Fatalf("edge_id: %#v", conn.lastExecArgs[0])
	}
}

func TestUpsertContextEdgeInvalid(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if err := d.UpsertContextEdge(context.Background(), ctxmodel.Edge{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpsertContextEdgeExecError(t *testing.T) {
	d := &DB{conn: &fakeConn{execErr: sql.ErrConnDone}}
	edge := ctxmodel.Edge{FromNodeID: "node_1", ToNodeID: "node_2", Relation: "depends_on"}
	if err := d.UpsertContextEdge(context.Background(), edge); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetServiceGraphOK(t *testing.T) {
	row := fakeRow{values: []any{[]byte(`{"nodes":[],"edges":[]}`)}}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	out, err := d.GetServiceGraph(context.Background(), "svc")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if string(out) == "" {
		t.Fatalf("empty output")
	}
	if !strings.Contains(conn.lastQuery, "context_nodes") || !strings.Contains(conn.lastQuery, "context_edges") {
		t.Fatalf("missing query")
	}
	if got := len(conn.lastArgs); got != 1 {
		t.Fatalf("args: %d", got)
	}
}

func TestGetServiceGraphInvalidService(t *testing.T) {
	d := &DB{conn: &fakeConn{}}
	if _, err := d.GetServiceGraph(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}

func TestGetServiceGraphRowError(t *testing.T) {
	row := fakeRow{err: sql.ErrConnDone}
	conn := &fakeConn{row: row}
	d := &DB{conn: conn}
	if _, err := d.GetServiceGraph(context.Background(), "svc"); err == nil {
		t.Fatalf("expected error")
	}
}
