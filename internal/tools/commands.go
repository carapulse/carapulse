package tools

import (
	"encoding/json"
	"fmt"
	"strings"
)

func BuildKubectlCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "scale":
		resource, _ := m["resource"].(string)
		replicas := intFromAny(m["replicas"])
		cmd := []string{"kubectl", "scale", resource, fmt.Sprintf("--replicas=%d", replicas)}
		if current, ok := intFromAnyOK(m["current_replicas"]); ok {
			cmd = append(cmd, fmt.Sprintf("--current-replicas=%d", current))
		}
		if rv, ok := m["resource_version"].(string); ok && rv != "" {
			cmd = append(cmd, fmt.Sprintf("--resource-version=%s", rv))
		}
		return cmd
	case "rollout-status":
		resource, _ := m["resource"].(string)
		return []string{"kubectl", "rollout", "status", resource}
	case "get":
		resource, _ := m["resource"].(string)
		cmd := []string{"kubectl", "get", resource, "-o", "json"}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "-n", ns)
		}
		if selector, ok := m["selector"].(string); ok && selector != "" {
			cmd = append(cmd, "--selector", selector)
		}
		return cmd
	case "watch":
		resource, _ := m["resource"].(string)
		cmd := []string{"kubectl", "get", resource, "-o", "json", "--watch", "--output-watch-events"}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "-n", ns)
		}
		if selector, ok := m["selector"].(string); ok && selector != "" {
			cmd = append(cmd, "--selector", selector)
		}
		if rv, ok := m["resource_version"].(string); ok && rv != "" {
			cmd = append(cmd, "--resource-version", rv)
		}
		if allow, ok := m["allow_bookmarks"].(bool); ok && allow {
			cmd = append(cmd, "--allow-watch-bookmarks")
		}
		if send, ok := m["send_initial_events"].(bool); ok && !send {
			cmd = append(cmd, "--watch-only")
		}
		if timeout, ok := intFromAnyOK(m["timeout_seconds"]); ok && timeout > 0 {
			cmd = append(cmd, fmt.Sprintf("--request-timeout=%ds", timeout))
		}
		return cmd
	default:
		return []string{"kubectl"}
	}
}

func BuildHelmCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "status":
		release, _ := m["release"].(string)
		cmd := []string{"helm", "status", release}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "--namespace", ns)
		}
		return cmd
	case "list":
		cmd := []string{"helm", "list", "-o", "json"}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "--namespace", ns)
		}
		return cmd
	case "get":
		release, _ := m["release"].(string)
		cmd := []string{"helm", "get", "all", release}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "--namespace", ns)
		}
		return cmd
	case "upgrade":
		release, _ := m["release"].(string)
		chart, _ := m["chart"].(string)
		valuesFile, _ := m["values_file"].(string)
		namespace, _ := m["namespace"].(string)
		if chart == "" {
			if valuesFile == "" {
				cmd := []string{"helm", "upgrade", "--install", release}
				if namespace != "" {
					cmd = append(cmd, "--namespace", namespace)
				}
				return cmd
			}
			cmd := []string{"helm", "upgrade", "--install", release, "-f", valuesFile}
			if namespace != "" {
				cmd = append(cmd, "--namespace", namespace)
			}
			return cmd
		}
		if valuesFile == "" {
			cmd := []string{"helm", "upgrade", "--install", release, chart}
			if namespace != "" {
				cmd = append(cmd, "--namespace", namespace)
			}
			return cmd
		}
		cmd := []string{"helm", "upgrade", "--install", release, chart, "-f", valuesFile}
		if namespace != "" {
			cmd = append(cmd, "--namespace", namespace)
		}
		return cmd
	case "rollback":
		release, _ := m["release"].(string)
		cmd := []string{"helm", "rollback", release}
		if ns, ok := m["namespace"].(string); ok && ns != "" {
			cmd = append(cmd, "--namespace", ns)
		}
		return cmd
	default:
		return []string{"helm"}
	}
}

func BuildArgoCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	app, _ := m["app"].(string)
	switch action {
	case "sync":
		return []string{"argocd", "app", "sync", app}
	case "sync-dry-run":
		return []string{"argocd", "app", "sync", app, "--dry-run"}
	case "sync-preview":
		return []string{"argocd", "app", "sync", app, "--dry-run", "--preview-changes"}
	case "wait":
		return []string{"argocd", "app", "wait", app, "--health"}
	case "rollback":
		revision, _ := m["revision"].(string)
		if revision != "" {
			return []string{"argocd", "app", "rollback", app, revision}
		}
		return []string{"argocd", "app", "rollback", app}
	case "status":
		return []string{"argocd", "app", "get", app, "-o", "json"}
	case "list":
		return []string{"argocd", "app", "list", "-o", "json"}
	case "project_token_create":
		project, _ := m["project"].(string)
		role, _ := m["role"].(string)
		return []string{"argocd", "proj", "role", "create-token", project, role}
	case "project_token_delete":
		project, _ := m["project"].(string)
		role, _ := m["role"].(string)
		tokenID, _ := m["token_id"].(string)
		return []string{"argocd", "proj", "role", "delete-token", project, role, tokenID}
	default:
		return []string{"argocd"}
	}
}

func BuildAwsCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	region, _ := m["region"].(string)
	withRegion := func(cmd []string) []string {
		if region != "" {
			cmd = append(cmd, "--region", region)
		}
		return cmd
	}
	withOutput := func(cmd []string) []string {
		if out, ok := m["output"].(string); ok && out != "" {
			cmd = append(cmd, "--output", out)
			return cmd
		}
		return append(cmd, "--output", "json")
	}
	switch action {
	case "assume-role":
		roleArn, _ := m["role_arn"].(string)
		sessionName, _ := m["session_name"].(string)
		if sessionName == "" {
			sessionName = "carapulse"
		}
		cmd := []string{"aws", "sts", "assume-role", "--role-arn", roleArn, "--role-session-name", sessionName}
		if dur, ok := intFromAnyOK(m["duration_seconds"]); ok && dur > 0 {
			cmd = append(cmd, fmt.Sprintf("--duration-seconds=%d", dur))
		}
		return withOutput(withRegion(cmd))
	case "tagging-get-resources":
		cmd := []string{"aws", "resourcegroupstaggingapi", "get-resources"}
		if filters, ok := m["tag_filters"].([]any); ok {
			for _, f := range filters {
				if fm, ok := f.(map[string]any); ok {
					key, _ := fm["key"].(string)
					values, _ := fm["values"].([]any)
					if key != "" && len(values) > 0 {
						var vals []string
						for _, v := range values {
							if s, ok := v.(string); ok && s != "" {
								vals = append(vals, s)
							}
						}
						if len(vals) > 0 {
							cmd = append(cmd, "--tag-filters", fmt.Sprintf("Key=%s,Values=%s", key, strings.Join(vals, ",")))
						}
					}
				}
			}
		}
		return withOutput(withRegion(cmd))
	case "cloudtrail-lookup-events":
		cmd := []string{"aws", "cloudtrail", "lookup-events"}
		if attrs, ok := m["lookup_attributes"].([]any); ok {
			for _, attr := range attrs {
				if am, ok := attr.(map[string]any); ok {
					key, _ := am["key"].(string)
					val, _ := am["value"].(string)
					if key != "" && val != "" {
						cmd = append(cmd, "--lookup-attributes", fmt.Sprintf("AttributeKey=%s,AttributeValue=%s", key, val))
					}
				}
			}
		}
		return withOutput(withRegion(cmd))
	case "cloudwatch-get-metric-data":
		cmd := []string{"aws", "cloudwatch", "get-metric-data"}
		if queries, ok := m["metric_data_queries"]; ok {
			if data, err := json.Marshal(queries); err == nil {
				cmd = append(cmd, "--metric-data-queries", string(data))
			}
		}
		return withOutput(withRegion(cmd))
	default:
		return []string{"aws", action}
	}
}

func BuildVaultCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "health":
		return []string{"vault", "status", "-format=json"}
	case "audit_enable":
		typ, _ := m["type"].(string)
		if typ == "" {
			typ = "file"
		}
		path, _ := m["path"].(string)
		cmd := []string{"vault", "audit", "enable", typ}
		if path != "" && strings.EqualFold(typ, "file") {
			cmd = append(cmd, "file_path="+path)
		}
		return cmd
	case "token_renew":
		return []string{"vault", "token", "renew"}
	default:
		return []string{"vault", action}
	}
}

func BuildBoundaryCmd(action string, input any) []string {
	_ = input
	return []string{"boundary", action}
}

func BuildGhCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "pr":
		cmd := []string{"gh", "pr", "create"}
		if title, ok := m["title"].(string); ok && title != "" {
			cmd = append(cmd, "--title", title)
		}
		if body, ok := m["body"].(string); ok && body != "" {
			cmd = append(cmd, "--body", body)
		}
		if base, ok := m["base"].(string); ok && base != "" {
			cmd = append(cmd, "--base", base)
		}
		if head, ok := m["head"].(string); ok && head != "" {
			cmd = append(cmd, "--head", head)
		}
		if draft, ok := m["draft"].(bool); ok && draft {
			cmd = append(cmd, "--draft")
		}
		if labels, ok := m["labels"].([]any); ok {
			for _, label := range labels {
				if s, ok := label.(string); ok && s != "" {
					cmd = append(cmd, "--label", s)
				}
			}
		}
		if reviewers, ok := m["reviewers"].([]any); ok {
			for _, reviewer := range reviewers {
				if s, ok := reviewer.(string); ok && s != "" {
					cmd = append(cmd, "--reviewer", s)
				}
			}
		}
		if len(cmd) == 3 {
			cmd = append(cmd, "--fill")
		}
		return cmd
	case "pr_status":
		return []string{"gh", "pr", "status", "--json", "number,url,headRefName,headRefOid,state"}
	case "pr_merge":
		pr, _ := m["pr"].(string)
		method, _ := m["method"].(string)
		cmd := []string{"gh", "pr", "merge", pr}
		switch strings.ToLower(method) {
		case "squash":
			cmd = append(cmd, "--squash")
		case "rebase":
			cmd = append(cmd, "--rebase")
		default:
			cmd = append(cmd, "--merge")
		}
		return cmd
	case "pr_view":
		pr, _ := m["pr"].(string)
		return []string{"gh", "pr", "view", pr, "--json", "number,url,headRefName,headRefOid,state,mergeCommit"}
	case "branch_create":
		repo, _ := m["repo"].(string)
		branch, _ := m["branch"].(string)
		sha, _ := m["sha"].(string)
		ref := "refs/heads/" + branch
		return []string{"gh", "api", "-X", "POST", "repos/" + repo + "/git/refs", "-f", "ref=" + ref, "-f", "sha=" + sha}
	case "commit_status":
		repo, _ := m["repo"].(string)
		sha, _ := m["sha"].(string)
		state, _ := m["state"].(string)
		context, _ := m["context"].(string)
		desc, _ := m["description"].(string)
		targetURL, _ := m["target_url"].(string)
		cmd := []string{"gh", "api", "-X", "POST", "repos/" + repo + "/statuses/" + sha, "-f", "state=" + state}
		if context != "" {
			cmd = append(cmd, "-f", "context="+context)
		}
		if desc != "" {
			cmd = append(cmd, "-f", "description="+desc)
		}
		if targetURL != "" {
			cmd = append(cmd, "-f", "target_url="+targetURL)
		}
		return cmd
	default:
		return []string{"gh", action}
	}
}

func BuildGlabCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "mr":
		cmd := []string{"glab", "mr", "create"}
		if title, ok := m["title"].(string); ok && title != "" {
			cmd = append(cmd, "--title", title)
		}
		if body, ok := m["body"].(string); ok && body != "" {
			cmd = append(cmd, "--description", body)
		}
		if source, ok := m["source"].(string); ok && source != "" {
			cmd = append(cmd, "--source-branch", source)
		}
		if target, ok := m["target"].(string); ok && target != "" {
			cmd = append(cmd, "--target-branch", target)
		}
		if draft, ok := m["draft"].(bool); ok && draft {
			cmd = append(cmd, "--draft")
		}
		if labels, ok := m["labels"].([]any); ok {
			for _, label := range labels {
				if s, ok := label.(string); ok && s != "" {
					cmd = append(cmd, "--label", s)
				}
			}
		}
		if len(cmd) == 3 {
			cmd = append(cmd, "--fill")
		}
		return cmd
	case "mr_status":
		mr, _ := m["mr"].(string)
		if mr == "" {
			return []string{"glab", "mr", "list"}
		}
		return []string{"glab", "mr", "view", mr, "--json"}
	case "mr_merge":
		mr, _ := m["mr"].(string)
		cmd := []string{"glab", "mr", "merge", mr}
		if squash, ok := m["squash"].(bool); ok && squash {
			cmd = append(cmd, "--squash")
		}
		return cmd
	case "mr_view":
		mr, _ := m["mr"].(string)
		return []string{"glab", "mr", "view", mr, "--json"}
	case "branch_create":
		project, _ := m["project"].(string)
		branch, _ := m["branch"].(string)
		ref, _ := m["ref"].(string)
		return []string{"glab", "api", "-X", "POST", "projects/" + project + "/repository/branches", "-f", "branch=" + branch, "-f", "ref=" + ref}
	case "commit_status":
		project, _ := m["project"].(string)
		sha, _ := m["sha"].(string)
		state, _ := m["state"].(string)
		context, _ := m["context"].(string)
		desc, _ := m["description"].(string)
		targetURL, _ := m["target_url"].(string)
		cmd := []string{"glab", "api", "-X", "POST", "projects/" + project + "/statuses/" + sha, "-f", "state=" + state}
		if context != "" {
			cmd = append(cmd, "-f", "context="+context)
		}
		if desc != "" {
			cmd = append(cmd, "-f", "description="+desc)
		}
		if targetURL != "" {
			cmd = append(cmd, "-f", "target_url="+targetURL)
		}
		return cmd
	default:
		return []string{"glab", action}
	}
}

func BuildGitCmd(action string, input any) []string {
	m, _ := input.(map[string]any)
	switch action {
	case "branch":
		name, _ := m["name"].(string)
		base, _ := m["base"].(string)
		cmd := []string{"git", "checkout", "-b", name}
		if base != "" {
			cmd = append(cmd, base)
		}
		return cmd
	case "add":
		cmd := []string{"git", "add"}
		if paths, ok := m["paths"].([]any); ok {
			for _, path := range paths {
				if s, ok := path.(string); ok && s != "" {
					cmd = append(cmd, s)
				}
			}
		}
		return cmd
	case "commit":
		message, _ := m["message"].(string)
		cmd := []string{"git", "commit"}
		if all, ok := m["all"].(bool); ok && all {
			cmd = append(cmd, "-a")
		}
		if message != "" {
			cmd = append(cmd, "-m", message)
		}
		if paths, ok := m["paths"].([]any); ok && len(paths) > 0 {
			cmd = append(cmd, "--")
			for _, path := range paths {
				if s, ok := path.(string); ok && s != "" {
					cmd = append(cmd, s)
				}
			}
		}
		return cmd
	case "push":
		remote, _ := m["remote"].(string)
		branch, _ := m["branch"].(string)
		cmd := []string{"git", "push"}
		if setUpstream, ok := m["set_upstream"].(bool); ok && setUpstream {
			cmd = append(cmd, "-u")
		}
		if remote != "" {
			cmd = append(cmd, remote)
		}
		if branch != "" {
			cmd = append(cmd, branch)
		}
		return cmd
	case "status":
		return []string{"git", "status", "--porcelain"}
	case "checkout":
		ref, _ := m["ref"].(string)
		return []string{"git", "checkout", ref}
	case "merge":
		ref, _ := m["ref"].(string)
		return []string{"git", "merge", ref}
	case "fetch":
		remote, _ := m["remote"].(string)
		ref, _ := m["ref"].(string)
		cmd := []string{"git", "fetch"}
		if remote != "" {
			cmd = append(cmd, remote)
		}
		if ref != "" {
			cmd = append(cmd, ref)
		}
		return cmd
	default:
		return []string{"git"}
	}
}

func intFromAny(v any) int {
	switch t := v.(type) {
	case int:
		return t
	case float64:
		return int(t)
	default:
		return 0
	}
}
