package web

import (
	"fmt"
	"net/http"
	"strings"
)

func (s *Server) handleUIPlaybooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, uiPage("Playbooks", "/v1/playbooks"))
}

func (s *Server) UIHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ui/playbooks", s.handleUIPlaybooks)
	mux.HandleFunc("/ui/runbooks", s.handleUIRunbooks)
	mux.HandleFunc("/ui/workflows", s.handleUIWorkflows)
	mux.HandleFunc("/ui/plans/", s.handleUIPlan)
	return mux
}

func (s *Server) handleUIRunbooks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, uiPage("Runbooks", "/v1/runbooks"))
}

func (s *Server) handleUIWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, uiPage("Workflows", "/v1/workflows"))
}

func (s *Server) handleUIPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	planID := strings.TrimPrefix(r.URL.Path, "/ui/plans/")
	planID = strings.Trim(planID, "/")
	if planID == "" {
		http.Error(w, "plan id required", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, uiPlanPage(planID))
}

func uiPage(title, endpoint string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>%s</title>
  <style>
    body { font-family: "SF Mono", ui-monospace, Menlo, Monaco, monospace; margin: 32px; background: #0f1115; color: #f4f4f5; }
    h1 { font-size: 22px; }
    .card { background: #151922; padding: 16px; border-radius: 8px; margin-bottom: 12px; }
    pre { white-space: pre-wrap; word-wrap: break-word; }
  </style>
</head>
<body>
  <h1>%s</h1>
  <div id="content">Loading...</div>
  <script>
    const params = new URLSearchParams(window.location.search);
    const tenant = params.get("tenant_id");
    const headers = {"Accept": "application/json"};
    if (tenant) {
      headers["X-Tenant-Id"] = tenant;
    }
    fetch("%s", {headers})
      .then(r => r.text())
      .then(text => {
        let data;
        try { data = JSON.parse(text); } catch (e) { data = text; }
        const el = document.getElementById("content");
        if (Array.isArray(data)) {
          el.innerHTML = data.map(item => "<div class=\"card\"><pre>" + JSON.stringify(item, null, 2) + "</pre></div>").join("");
        } else if (data && data.workflows) {
          el.innerHTML = data.workflows.map(item => "<div class=\"card\"><pre>" + JSON.stringify(item, null, 2) + "</pre></div>").join("");
        } else {
          el.innerHTML = "<div class=\"card\"><pre>" + JSON.stringify(data, null, 2) + "</pre></div>";
        }
      })
      .catch(err => {
        document.getElementById("content").innerText = err.toString();
      });
  </script>
</body>
</html>`, title, title, endpoint)
}

func uiPlanPage(planID string) string {
	return fmt.Sprintf(`<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <title>Plan %s</title>
  <style>
    body { font-family: "SF Mono", ui-monospace, Menlo, Monaco, monospace; margin: 32px; background: #0f1115; color: #f4f4f5; }
    h1 { font-size: 22px; }
    .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
    .card { background: #151922; padding: 16px; border-radius: 8px; min-height: 200px; }
    pre { white-space: pre-wrap; word-wrap: break-word; }
  </style>
</head>
<body>
  <h1>Plan %s</h1>
  <div class="grid">
    <div class="card"><h3>Diff</h3><pre id="diff">Loading...</pre></div>
    <div class="card"><h3>Risk</h3><pre id="risk">Loading...</pre></div>
  </div>
  <script>
    fetch("/v1/plans/%s/diff").then(r => r.text()).then(t => {document.getElementById("diff").innerText = t;});
    fetch("/v1/plans/%s/risk").then(r => r.text()).then(t => {document.getElementById("risk").innerText = t;});
  </script>
</body>
</html>`, planID, planID, planID, planID)
}
