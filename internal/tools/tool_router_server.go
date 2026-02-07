package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"carapulse/internal/policy"
)

type Server struct {
	Router  *Router
	Sandbox *Sandbox
	Clients HTTPClients
	Auth    AuthConfig
	Policy  *policy.Evaluator
}

type ListToolsResponse struct {
	Tools []Tool `json:"tools"`
}

type ResolveResourceRequest struct {
	Artifact ArtifactRef `json:"artifact"`
}

type ResolveResourceResponse struct {
	Data []byte `json:"data"`
}

var resolveArtifact = ResolveArtifact
var marshalLogJSON = json.Marshal

func NewServer(router *Router, sandbox *Sandbox, clients HTTPClients) *Server {
	return &Server{Router: router, Sandbox: sandbox, Clients: clients}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch r.URL.Path {
	case "/", "/v1/tools:execute", "/execute":
		s.handleExecute(w, r)
	case "/v1/tools", "/tools":
		s.handleListTools(w, r)
	case "/v1/resources", "/resources":
		s.handleListResources(w, r)
	case "/v1/prompts", "/prompts":
		s.handleListPrompts(w, r)
	case "/v1/resources:resolve", "/resources:resolve":
		s.handleResolveResource(w, r)
	case "/v1/tools/logs", "/tools/logs":
		s.handleToolLogs(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) authenticate(r *http.Request) (JWTPayload, error) {
	token, err := ParseBearer(r)
	if err != nil || token == "" {
		return JWTPayload{}, errors.New("unauthorized")
	}
	if s.Auth.Token != "" && token == s.Auth.Token {
		return JWTPayload{Sub: "service", Email: "service", Groups: []string{"service"}}, nil
	}
	claims, err := ParseJWTClaims(token)
	if err != nil {
		return JWTPayload{}, err
	}
	if err := validateClaims(claims, s.Auth); err != nil {
		return JWTPayload{}, err
	}
	if err := VerifyJWTSignature(token, s.Auth); err != nil {
		return JWTPayload{}, err
	}
	return claims, nil
}

func (s *Server) authorize(ctx *http.Request, tool, action string, ctxRef ContextRef) error {
	if s.Policy == nil || s.Policy.Checker == nil {
		return errors.New("policy required")
	}
	actionType := actionTypeForTool(tool, action)
	risk := riskForToolAction(tool, action)
	tier := tierForRisk(risk)
	blast := blastRadiusForContext(ctxRef)
	breakGlass := strings.EqualFold(strings.TrimSpace(ctx.Header.Get("X-Break-Glass")), "true")
	if err := validateContextRefStrict(ctxRef); err != nil {
		return err
	}
	actor := map[string]any{
		"id":         ctx.Context().Value(actorIDKey),
		"email":      ctx.Context().Value(actorEmailKey),
		"roles":      ctx.Context().Value(actorRolesKey),
		"tenant_id":  ctx.Context().Value(actorTenantKey),
		"session_id": ctx.Context().Value(sessionIDKey),
	}
	dec, err := s.Policy.Check(ctx.Context(), policy.PolicyInput{
		Actor:     actor,
		Action:    policy.Action{Name: "tool.execute", Type: actionType},
		Context:   ctxRef,
		Risk:      policy.Risk{Level: risk, Tier: tier, BlastRadius: blast},
		Time:      time.Now().UTC().Format(time.RFC3339),
		Resources: map[string]any{"break_glass": breakGlass},
	})
	if err != nil {
		if actionType == "read" {
			return nil
		}
		return err
	}
	if risk == "high" && !breakGlass {
		return errors.New("break-glass required")
	}
	switch dec.Decision {
	case "", "allow":
		return nil
	default:
		return errors.New("policy decision " + dec.Decision)
	}
}

func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	claims, err := s.authenticate(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	ctx := r.Context()
	ctx = contextWithActor(ctx, claims)
	if sessionID := strings.TrimSpace(r.Header.Get("X-Session-Id")); sessionID != "" {
		ctx = context.WithValue(ctx, sessionIDKey, sessionID)
	}
	r = r.WithContext(ctx)
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := s.authorize(r, req.Tool, req.Action, req.Context); err != nil {
		http.Error(w, "policy denied", http.StatusForbidden)
		return
	}
	resp, err := s.Router.Execute(r.Context(), req, s.Sandbox, s.Clients)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(ListToolsResponse{Tools: Registry})
}

func (s *Server) handleResolveResource(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req ResolveResourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	data, err := resolveArtifact(req.Artifact)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotImplemented):
			http.Error(w, err.Error(), http.StatusNotImplemented)
		case errors.Is(err, ErrUnsupportedArtifact), errors.Is(err, ErrInvalidArtifact):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}
	_ = json.NewEncoder(w).Encode(ResolveResourceResponse{Data: data})
}

func (s *Server) handleListResources(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"resources": ListResources()})
}

func (s *Server) handleListPrompts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"prompts": ListPrompts()})
}

func (s *Server) handleToolLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if _, err := s.authenticate(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if s.Router == nil {
		http.Error(w, "router required", http.StatusServiceUnavailable)
		return
	}
	toolCallID := strings.TrimSpace(r.URL.Query().Get("tool_call_id"))
	if toolCallID == "" {
		http.Error(w, "tool_call_id required", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "stream unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	_, _ = fmt.Fprint(w, ":ok\n\n")
	flusher.Flush()

	writeLine := func(line LogLine) bool {
		payload, err := marshalLogJSON(line)
		if err != nil {
			return false
		}
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
		return true
	}

	hub := s.Router.logHub()
	for _, line := range hub.History(toolCallID) {
		if ok := writeLine(line); !ok {
			return
		}
	}
	ch, cancel := hub.Subscribe(toolCallID)
	defer cancel()
	for {
		select {
		case <-r.Context().Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if ok := writeLine(line); !ok {
				return
			}
		}
	}
}
