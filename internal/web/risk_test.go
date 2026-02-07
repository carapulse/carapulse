package web

import "testing"

func TestTierForRisk(t *testing.T) {
	if tierForRisk("read") != "read" {
		t.Fatalf("read")
	}
	if tierForRisk("low") != "safe" {
		t.Fatalf("low")
	}
	if tierForRisk("high") != "break_glass" {
		t.Fatalf("high")
	}
}

func TestBlastRadius(t *testing.T) {
	ctx := ContextRef{Namespace: "ns"}
	if blastRadius(ctx, 1) != "namespace" {
		t.Fatalf("namespace")
	}
	if blastRadius(ContextRef{}, 20) != "cluster" {
		t.Fatalf("cluster")
	}
	if blastRadius(ContextRef{}, 100) != "account" {
		t.Fatalf("account")
	}
}
