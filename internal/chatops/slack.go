package chatops

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"carapulse/internal/web"
)

type GatewayClient interface {
	CreatePlan(ctx context.Context, req web.PlanCreateRequest) (string, error)
	CreateApproval(ctx context.Context, planID, status, note string) error
	GetExecution(ctx context.Context, executionID string) ([]byte, error)
	ListAudit(ctx context.Context, query url.Values) ([]byte, error)
}

type SlackHandler struct {
	SigningSecret string
	Client        GatewayClient
	Clock         func() time.Time
}

var readAll = io.ReadAll

func NewSlackHandler(secret string, client GatewayClient) *SlackHandler {
	return &SlackHandler{SigningSecret: secret, Client: client, Clock: time.Now}
}

func (h *SlackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if h.Client == nil {
		http.Error(w, "client required", http.StatusInternalServerError)
		return
	}
	body, err := readAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}
	if !h.verifySignature(r.Header, body) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		http.Error(w, "invalid body", http.StatusBadRequest)
		return
	}
	text := strings.TrimSpace(values.Get("text"))
	action, rest := splitFirst(text)
	if action == "" {
		http.Error(w, "missing command", http.StatusBadRequest)
		return
	}
	switch action {
	case "plan":
		h.handlePlan(w, r, strings.TrimSpace(rest))
	case "approve":
		h.handleApprove(w, r, strings.TrimSpace(rest))
	case "status":
		h.handleStatus(w, r, strings.TrimSpace(rest))
	case "audit":
		h.handleAudit(w, r, strings.TrimSpace(rest))
	default:
		http.Error(w, "unknown command", http.StatusBadRequest)
	}
}

func (h *SlackHandler) handlePlan(w http.ResponseWriter, r *http.Request, text string) {
	if text == "" {
		http.Error(w, "missing intent", http.StatusBadRequest)
		return
	}
	req := web.PlanCreateRequest{
		Summary: text,
		Trigger: "manual",
		Intent:  text,
		Context: web.ContextRef{},
	}
	planID, err := h.Client.CreatePlan(r.Context(), req)
	if err != nil {
		http.Error(w, "plan error", http.StatusBadRequest)
		return
	}
	writeText(w, fmt.Sprintf("plan created: %s", planID))
}

func (h *SlackHandler) handleApprove(w http.ResponseWriter, r *http.Request, text string) {
	planID, rest := splitFirst(text)
	if planID == "" {
		http.Error(w, "missing plan_id", http.StatusBadRequest)
		return
	}
	status := "approved"
	if rest != "" {
		status = rest
	}
	if err := h.Client.CreateApproval(r.Context(), planID, status, ""); err != nil {
		http.Error(w, "approval error", http.StatusBadRequest)
		return
	}
	writeText(w, fmt.Sprintf("approval %s: %s", status, planID))
}

func (h *SlackHandler) handleStatus(w http.ResponseWriter, r *http.Request, text string) {
	if text == "" {
		http.Error(w, "missing execution_id", http.StatusBadRequest)
		return
	}
	resp, err := h.Client.GetExecution(r.Context(), text)
	if err != nil {
		http.Error(w, "status error", http.StatusBadRequest)
		return
	}
	writeText(w, string(resp))
}

func (h *SlackHandler) handleAudit(w http.ResponseWriter, r *http.Request, text string) {
	values := url.Values{}
	if text != "" {
		values.Set("plan_id", text)
	}
	resp, err := h.Client.ListAudit(r.Context(), values)
	if err != nil {
		http.Error(w, "audit error", http.StatusBadRequest)
		return
	}
	writeText(w, string(resp))
}

func (h *SlackHandler) verifySignature(header http.Header, body []byte) bool {
	if h.SigningSecret == "" {
		return false
	}
	ts := header.Get("X-Slack-Request-Timestamp")
	sig := header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return false
	}
	if h.Clock == nil {
		h.Clock = time.Now
	}
	parsed, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	now := h.Clock().Unix()
	if now-parsed > 60*5 || parsed-now > 60*5 {
		return false
	}
	base := fmt.Sprintf("v0:%s:%s", ts, string(body))
	mac := hmac.New(sha256.New, []byte(h.SigningSecret))
	_, _ = mac.Write([]byte(base))
	sum := mac.Sum(nil)
	expected := "v0=" + hex.EncodeToString(sum)
	return hmac.Equal([]byte(expected), []byte(sig))
}

func splitFirst(text string) (string, string) {
	if text == "" {
		return "", ""
	}
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return "", ""
	}
	action := parts[0]
	rest := strings.TrimSpace(strings.TrimPrefix(text, action))
	return action, rest
}

func writeText(w http.ResponseWriter, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = io.WriteString(w, text)
}

var errClient = errors.New("client error")
