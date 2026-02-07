package tools

import "testing"

func TestBuildKubectlCmdScale(t *testing.T) {
	cmd := BuildKubectlCmd("scale", map[string]any{"resource": "deploy/app", "replicas": 3})
	want := []string{"kubectl", "scale", "deploy/app", "--replicas=3"}
	assertSlice(t, cmd, want)
}

func TestBuildKubectlCmdScalePreconditions(t *testing.T) {
	cmd := BuildKubectlCmd("scale", map[string]any{
		"resource":         "deploy/app",
		"replicas":         2,
		"current_replicas": 1,
		"resource_version": "10",
	})
	want := []string{"kubectl", "scale", "deploy/app", "--replicas=2", "--current-replicas=1", "--resource-version=10"}
	assertSlice(t, cmd, want)
}

func TestBuildKubectlCmdRollout(t *testing.T) {
	cmd := BuildKubectlCmd("rollout-status", map[string]any{"resource": "deploy/app"})
	want := []string{"kubectl", "rollout", "status", "deploy/app"}
	assertSlice(t, cmd, want)
}

func TestBuildKubectlCmdGet(t *testing.T) {
	cmd := BuildKubectlCmd("get", map[string]any{"resource": "deploy/app"})
	want := []string{"kubectl", "get", "deploy/app", "-o", "json"}
	assertSlice(t, cmd, want)
}

func TestBuildKubectlCmdWatch(t *testing.T) {
	cmd := BuildKubectlCmd("watch", map[string]any{
		"resource":            "deploy/app",
		"namespace":           "ns",
		"resource_version":    "10",
		"allow_bookmarks":     true,
		"send_initial_events": false,
		"timeout_seconds":     30,
		"selector":            "app=demo",
	})
	want := []string{"kubectl", "get", "deploy/app", "-o", "json", "--watch", "--output-watch-events", "-n", "ns", "--selector", "app=demo", "--resource-version", "10", "--allow-watch-bookmarks", "--watch-only", "--request-timeout=30s"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdUpgrade(t *testing.T) {
	cmd := BuildHelmCmd("upgrade", map[string]any{"release": "svc", "chart": "chart"})
	want := []string{"helm", "upgrade", "--install", "svc", "chart"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdUpgradeValuesFile(t *testing.T) {
	cmd := BuildHelmCmd("upgrade", map[string]any{"release": "svc", "chart": "chart", "values_file": "/tmp/values.yaml"})
	want := []string{"helm", "upgrade", "--install", "svc", "chart", "-f", "/tmp/values.yaml"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdRollback(t *testing.T) {
	cmd := BuildHelmCmd("rollback", map[string]any{"release": "svc"})
	want := []string{"helm", "rollback", "svc"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdUpgradeNoChart(t *testing.T) {
	cmd := BuildHelmCmd("upgrade", map[string]any{"release": "svc"})
	want := []string{"helm", "upgrade", "--install", "svc"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdUpgradeNoChartValuesFile(t *testing.T) {
	cmd := BuildHelmCmd("upgrade", map[string]any{"release": "svc", "values_file": "/tmp/values.yaml"})
	want := []string{"helm", "upgrade", "--install", "svc", "-f", "/tmp/values.yaml"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdStatus(t *testing.T) {
	cmd := BuildHelmCmd("status", map[string]any{"release": "svc"})
	want := []string{"helm", "status", "svc"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdList(t *testing.T) {
	cmd := BuildHelmCmd("list", map[string]any{})
	want := []string{"helm", "list", "-o", "json"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdListNamespace(t *testing.T) {
	cmd := BuildHelmCmd("list", map[string]any{"namespace": "ns"})
	want := []string{"helm", "list", "-o", "json", "--namespace", "ns"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdGet(t *testing.T) {
	cmd := BuildHelmCmd("get", map[string]any{"release": "svc"})
	want := []string{"helm", "get", "all", "svc"}
	assertSlice(t, cmd, want)
}

func TestBuildHelmCmdDefault(t *testing.T) {
	cmd := BuildHelmCmd("unknown", map[string]any{})
	want := []string{"helm"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdSync(t *testing.T) {
	cmd := BuildArgoCmd("sync", map[string]any{"app": "svc"})
	want := []string{"argocd", "app", "sync", "svc"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdWait(t *testing.T) {
	cmd := BuildArgoCmd("wait", map[string]any{"app": "svc"})
	want := []string{"argocd", "app", "wait", "svc", "--health"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdSyncDryRun(t *testing.T) {
	cmd := BuildArgoCmd("sync-dry-run", map[string]any{"app": "svc"})
	want := []string{"argocd", "app", "sync", "svc", "--dry-run"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdSyncPreview(t *testing.T) {
	cmd := BuildArgoCmd("sync-preview", map[string]any{"app": "svc"})
	want := []string{"argocd", "app", "sync", "svc", "--dry-run", "--preview-changes"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdRollback(t *testing.T) {
	cmd := BuildArgoCmd("rollback", map[string]any{"app": "svc", "revision": "3"})
	want := []string{"argocd", "app", "rollback", "svc", "3"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdStatus(t *testing.T) {
	cmd := BuildArgoCmd("status", map[string]any{"app": "svc"})
	want := []string{"argocd", "app", "get", "svc", "-o", "json"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdList(t *testing.T) {
	cmd := BuildArgoCmd("list", map[string]any{})
	want := []string{"argocd", "app", "list", "-o", "json"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdProjectTokenCreate(t *testing.T) {
	cmd := BuildArgoCmd("project_token_create", map[string]any{"project": "proj", "role": "role"})
	want := []string{"argocd", "proj", "role", "create-token", "proj", "role"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdProjectTokenDelete(t *testing.T) {
	cmd := BuildArgoCmd("project_token_delete", map[string]any{"project": "proj", "role": "role", "token_id": "t1"})
	want := []string{"argocd", "proj", "role", "delete-token", "proj", "role", "t1"}
	assertSlice(t, cmd, want)
}

func TestBuildArgoCmdDefault(t *testing.T) {
	cmd := BuildArgoCmd("unknown", map[string]any{})
	want := []string{"argocd"}
	assertSlice(t, cmd, want)
}

func TestBuildKubectlCmdDefault(t *testing.T) {
	cmd := BuildKubectlCmd("unknown", map[string]any{})
	want := []string{"kubectl"}
	assertSlice(t, cmd, want)
}

func TestBuildCommandsDefault(t *testing.T) {
	if got := BuildAwsCmd("sts", nil); got[0] != "aws" {
		t.Fatalf("aws cmd")
	}
	if got := BuildVaultCmd("status", nil); got[0] != "vault" {
		t.Fatalf("vault cmd")
	}
	if got := BuildBoundaryCmd("status", nil); got[0] != "boundary" {
		t.Fatalf("boundary cmd")
	}
	if got := BuildGhCmd("status", nil); got[0] != "gh" {
		t.Fatalf("gh cmd")
	}
	if got := BuildGlabCmd("status", nil); got[0] != "glab" {
		t.Fatalf("glab cmd")
	}
	if got := BuildGitCmd("status", nil); got[0] != "git" {
		t.Fatalf("git cmd")
	}
}

func TestBuildGhCmdPR(t *testing.T) {
	cmd := BuildGhCmd("pr", map[string]any{
		"title":     "feat",
		"body":      "desc",
		"base":      "main",
		"head":      "branch",
		"draft":     true,
		"labels":    []any{"bug"},
		"reviewers": []any{"alice"},
	})
	want := []string{"gh", "pr", "create", "--title", "feat", "--body", "desc", "--base", "main", "--head", "branch", "--draft", "--label", "bug", "--reviewer", "alice"}
	assertSlice(t, cmd, want)
}

func TestBuildGlabCmdMR(t *testing.T) {
	cmd := BuildGlabCmd("mr", map[string]any{
		"title":  "feat",
		"body":   "desc",
		"source": "branch",
		"target": "main",
		"draft":  true,
		"labels": []any{"bug"},
	})
	want := []string{"glab", "mr", "create", "--title", "feat", "--description", "desc", "--source-branch", "branch", "--target-branch", "main", "--draft", "--label", "bug"}
	assertSlice(t, cmd, want)
}

func TestBuildGitCmdBranch(t *testing.T) {
	cmd := BuildGitCmd("branch", map[string]any{"name": "feat", "base": "main"})
	want := []string{"git", "checkout", "-b", "feat", "main"}
	assertSlice(t, cmd, want)
}

func TestBuildGitCmdAdd(t *testing.T) {
	cmd := BuildGitCmd("add", map[string]any{"paths": []any{"a.txt", "b.txt"}})
	want := []string{"git", "add", "a.txt", "b.txt"}
	assertSlice(t, cmd, want)
}

func TestBuildGitCmdCommit(t *testing.T) {
	cmd := BuildGitCmd("commit", map[string]any{"message": "feat", "all": true})
	want := []string{"git", "commit", "-a", "-m", "feat"}
	assertSlice(t, cmd, want)
}

func TestBuildGitCmdPush(t *testing.T) {
	cmd := BuildGitCmd("push", map[string]any{"remote": "origin", "branch": "feat", "set_upstream": true})
	want := []string{"git", "push", "-u", "origin", "feat"}
	assertSlice(t, cmd, want)
}

func TestBuildAwsCmdAssumeRole(t *testing.T) {
	cmd := BuildAwsCmd("assume-role", map[string]any{"role_arn": "arn", "session_name": "sess", "duration_seconds": 900, "region": "us-east-1"})
	if cmd[0] != "aws" || cmd[1] != "sts" {
		t.Fatalf("aws assume")
	}
}

func TestBuildAwsCmdTagging(t *testing.T) {
	cmd := BuildAwsCmd("tagging-get-resources", map[string]any{
		"tag_filters": []any{map[string]any{"key": "env", "values": []any{"prod"}}},
	})
	if cmd[0] != "aws" || cmd[1] != "resourcegroupstaggingapi" {
		t.Fatalf("aws tagging")
	}
}

func TestBuildAwsCmdCloudTrail(t *testing.T) {
	cmd := BuildAwsCmd("cloudtrail-lookup-events", map[string]any{
		"lookup_attributes": []any{map[string]any{"key": "EventName", "value": "Update"}},
	})
	if cmd[0] != "aws" || cmd[1] != "cloudtrail" {
		t.Fatalf("aws cloudtrail")
	}
}

func TestBuildAwsCmdCloudWatch(t *testing.T) {
	cmd := BuildAwsCmd("cloudwatch-get-metric-data", map[string]any{
		"metric_data_queries": []any{map[string]any{"id": "m1"}},
	})
	if cmd[0] != "aws" || cmd[1] != "cloudwatch" {
		t.Fatalf("aws cloudwatch")
	}
}

func TestIntFromAny(t *testing.T) {
	if v := intFromAny(2.9); v != 2 {
		t.Fatalf("float: %d", v)
	}
	if v := intFromAny("x"); v != 0 {
		t.Fatalf("default: %d", v)
	}
}

func assertSlice(t *testing.T, got, want []string) {
	if len(got) != len(want) {
		t.Fatalf("len mismatch: %v vs %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("idx %d: %s != %s", i, got[i], want[i])
		}
	}
}
