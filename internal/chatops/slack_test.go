package chatops

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"carapulse/internal/web"
)

type fakeGateway struct {
	planID     string
	approvals  []string
	execBody   []byte
	auditBody  []byte
	err        error
	lastIntent string
}

func (f *fakeGateway) CreatePlan(ctx context.Context, req web.PlanCreateRequest) (string, error) {
	f.lastIntent = req.Intent
	if f.err != nil {
		return "", f.err
	}
	if f.planID == "" {
		f.planID = "plan_1"
	}
	return f.planID, nil
}

func (f *fakeGateway) CreateApproval(ctx context.Context, planID, status, note string) error {
	if f.err != nil {
		return f.err
	}
	f.approvals = append(f.approvals, planID+":"+status)
	return nil
}

func (f *fakeGateway) GetExecution(ctx context.Context, executionID string) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.execBody == nil {
		f.execBody = []byte(`{"execution_id":"` + executionID + `"}`)
	}
	return f.execBody, nil
}

func (f *fakeGateway) ListAudit(ctx context.Context, query url.Values) ([]byte, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.auditBody == nil {
		f.auditBody = []byte(`[]`)
	}
	return f.auditBody, nil
}

func signSlack(secret string, ts string, body []byte) string {
	base := "v0:" + ts + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(base))
	return "v0=" + hex.EncodeToString(mac.Sum(nil))
}

func newSlackRequest(t *testing.T, secret, text string) *http.Request {
	body := []byte("text=" + url.QueryEscape(text))
	ts := strconvNow()
	sig := signSlack(secret, ts, body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	return req
}

func strconvNow() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func TestSlackHandlerMethodNotAllowed(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerMissingClient(t *testing.T) {
	h := NewSlackHandler("secret", nil)
	req := newSlackRequest(t, "secret", "plan deploy app")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerBadSignature(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader([]byte("text=plan")))
	req.Header.Set("X-Slack-Request-Timestamp", strconvNow())
	req.Header.Set("X-Slack-Signature", "v0=bad")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerOldTimestamp(t *testing.T) {
	gw := &fakeGateway{}
	h := NewSlackHandler("secret", gw)
	h.Clock = func() time.Time { return time.Unix(1000, 0) }
	body := []byte("text=plan")
	ts := "1"
	sig := signSlack("secret", ts, body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerParseError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	body := []byte("%")
	ts := strconvNow()
	sig := signSlack("secret", ts, body)
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	req.Header.Set("X-Slack-Request-Timestamp", ts)
	req.Header.Set("X-Slack-Signature", sig)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerMissingCommand(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := newSlackRequest(t, "secret", "")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerUnknownCommand(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := newSlackRequest(t, "secret", "noop")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerPlan(t *testing.T) {
	gw := &fakeGateway{planID: "plan_1"}
	h := NewSlackHandler("secret", gw)
	req := newSlackRequest(t, "secret", "plan deploy app")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if gw.lastIntent == "" {
		t.Fatalf("missing intent")
	}
}

func TestSlackHandlerPlanMissingIntent(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := newSlackRequest(t, "secret", "plan")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerPlanError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{err: errClient})
	req := newSlackRequest(t, "secret", "plan deploy app")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerApproveDefault(t *testing.T) {
	gw := &fakeGateway{}
	h := NewSlackHandler("secret", gw)
	req := newSlackRequest(t, "secret", "approve plan_1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if len(gw.approvals) != 1 || !strings.Contains(gw.approvals[0], "approved") {
		t.Fatalf("approvals: %v", gw.approvals)
	}
}

func TestSlackHandlerApproveStatus(t *testing.T) {
	gw := &fakeGateway{}
	h := NewSlackHandler("secret", gw)
	req := newSlackRequest(t, "secret", "approve plan_1 denied")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if !strings.Contains(gw.approvals[0], "denied") {
		t.Fatalf("approvals: %v", gw.approvals)
	}
}

func TestSlackHandlerApproveMissingPlan(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := newSlackRequest(t, "secret", "approve")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerApproveError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{err: errClient})
	req := newSlackRequest(t, "secret", "approve plan_1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerStatus(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{execBody: []byte("ok")})
	req := newSlackRequest(t, "secret", "status exec_1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
	if w.Body.String() != "ok" {
		t.Fatalf("body: %s", w.Body.String())
	}
}

func TestSlackHandlerStatusMissing(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	req := newSlackRequest(t, "secret", "status")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerStatusError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{err: errClient})
	req := newSlackRequest(t, "secret", "status exec")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerAudit(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{auditBody: []byte("[]")})
	req := newSlackRequest(t, "secret", "audit plan_1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerAuditNoArg(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{auditBody: []byte("[]")})
	req := newSlackRequest(t, "secret", "audit")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerAuditError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{err: errClient})
	req := newSlackRequest(t, "secret", "audit plan_1")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestSlackHandlerReadError(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	oldRead := readAll
	readAll = func(r io.Reader) ([]byte, error) { return nil, errClient }
	defer func() { readAll = oldRead }()
	req := newSlackRequest(t, "secret", "plan deploy")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestVerifySignatureEmptySecret(t *testing.T) {
	h := NewSlackHandler("", &fakeGateway{})
	if h.verifySignature(http.Header{}, []byte("body")) {
		t.Fatalf("expected false")
	}
}

func TestVerifySignatureBadTimestamp(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	header := http.Header{}
	header.Set("X-Slack-Request-Timestamp", "bad")
	header.Set("X-Slack-Signature", "v0=bad")
	if h.verifySignature(header, []byte("body")) {
		t.Fatalf("expected false")
	}
}

func TestVerifySignatureMissingHeaders(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	if h.verifySignature(http.Header{}, []byte("body")) {
		t.Fatalf("expected false")
	}
}

func TestVerifySignatureClockNil(t *testing.T) {
	h := NewSlackHandler("secret", &fakeGateway{})
	h.Clock = nil
	ts := strconvNow()
	body := []byte("text=plan")
	header := http.Header{}
	header.Set("X-Slack-Request-Timestamp", ts)
	header.Set("X-Slack-Signature", signSlack("secret", ts, body))
	if !h.verifySignature(header, body) {
		t.Fatalf("expected true")
	}
}

func TestSplitFirstEmpty(t *testing.T) {
	if a, b := splitFirst(""); a != "" || b != "" {
		t.Fatalf("unexpected: %q %q", a, b)
	}
}

func TestSplitFirstWhitespace(t *testing.T) {
	if a, b := splitFirst("   "); a != "" || b != "" {
		t.Fatalf("unexpected: %q %q", a, b)
	}
}

func TestSplitFirstRest(t *testing.T) {
	if a, b := splitFirst("plan deploy app"); a != "plan" || b != "deploy app" {
		t.Fatalf("unexpected: %q %q", a, b)
	}
}
