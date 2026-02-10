package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

const testToken = "Bearer aaa.eyJzdWIiOiJ1IiwiZW1haWwiOiJ1QGV4YW1wbGUuY29tIiwiZ3JvdXBzIjpbInNyZSJdfQ.bbb"

type noFlushWriter struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (w *noFlushWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *noFlushWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

func (w *noFlushWriter) WriteHeader(status int) {
	w.status = status
}

type fakeHijacker struct{}

func (f fakeHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("no hijack")
}

type hijackWriter struct {
	header http.Header
	status int
}

func (w *hijackWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *hijackWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *hijackWriter) WriteHeader(status int) {
	w.status = status
}

func (w *hijackWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, fmt.Errorf("boom")
}

type errorWriter struct{}

func (e errorWriter) Write(p []byte) (int, error) {
	return 0, fmt.Errorf("write error")
}

type flushHijacker struct{}

func (f flushHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	serverConn, clientConn := net.Pipe()
	_ = clientConn.Close()
	rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriterSize(errorWriter{}, 1))
	return serverConn, rw, nil
}

type pipeWSWriter struct {
	conn   net.Conn
	rw     *bufio.ReadWriter
	header http.Header
	status int
}

func newPipeWSWriter() (*pipeWSWriter, net.Conn) {
	serverConn, clientConn := net.Pipe()
	rw := bufio.NewReadWriter(bufio.NewReader(serverConn), bufio.NewWriter(serverConn))
	return &pipeWSWriter{conn: serverConn, rw: rw}, clientConn
}

func (w *pipeWSWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *pipeWSWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (w *pipeWSWriter) WriteHeader(status int) {
	w.status = status
}

func (w *pipeWSWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return w.conn, w.rw, nil
}

func TestHandleWSMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/ws", nil)
	w := httptest.NewRecorder()
	srv := &Server{}
	srv.handleWS(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWSNoHijacker(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	w := httptest.NewRecorder()
	srv := &Server{}
	srv.handleWS(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleWSBadHandshake(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	w := &hijackWriter{}
	srv := &Server{}
	srv.handleWS(w, req)
	if w.status != http.StatusBadRequest {
		t.Fatalf("status: %d", w.status)
	}
}

func TestUpgradeWebSocketMissingKey(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	if _, _, err := upgradeWebSocket(req, fakeHijacker{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpgradeWebSocketNotWebSocket(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	if _, _, err := upgradeWebSocket(req, fakeHijacker{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpgradeWebSocketHijackError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	if _, _, err := upgradeWebSocket(req, fakeHijacker{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestUpgradeWebSocketFlushError(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	if _, _, err := upgradeWebSocket(req, flushHijacker{}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHeaderHasToken(t *testing.T) {
	h := http.Header{}
	h.Add("Connection", "keep-alive, Upgrade")
	if !headerHasToken(h, "Connection", "upgrade") {
		t.Fatalf("expected token")
	}
}

func TestHeaderHasTokenFalse(t *testing.T) {
	h := http.Header{}
	if headerHasToken(h, "Connection", "upgrade") {
		t.Fatalf("unexpected token")
	}
}

func TestIsWebSocketRequestFalse(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	if isWebSocketRequest(req) {
		t.Fatalf("expected false")
	}
}

func TestIsWebSocketRequestWrongUpgrade(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "h2c")
	if isWebSocketRequest(req) {
		t.Fatalf("expected false")
	}
}

func TestWriteTextFrameLengths(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	rw := bufio.NewReadWriter(bufio.NewReader(buf), bufio.NewWriter(buf))
	if err := writeTextFrame(rw, []byte(strings.Repeat("a", 2))); err != nil {
		t.Fatalf("short: %v", err)
	}
	if err := writeTextFrame(rw, []byte(strings.Repeat("b", 130))); err != nil {
		t.Fatalf("mid: %v", err)
	}
	if err := writeTextFrame(rw, []byte(strings.Repeat("c", 70000))); err != nil {
		t.Fatalf("long: %v", err)
	}
}

func TestWriteTextFrameError(t *testing.T) {
	rw := bufio.NewReadWriter(bufio.NewReader(bytes.NewBuffer(nil)), bufio.NewWriterSize(errorWriter{}, 1))
	if err := writeTextFrame(rw, []byte("hi")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestHandleWSHandshakeAndEvent(t *testing.T) {
	server := NewServer(&fakeDB{}, nil)
	ts := httptest.NewServer(server.Mux)
	defer ts.Close()

	u, err := url.Parse(ts.URL)
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	conn, err := net.Dial("tcp", u.Host)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	reader := bufio.NewReader(conn)

	fmt.Fprintf(conn, "GET /v1/ws HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\nAuthorization: %s\r\n\r\n", u.Host, testToken)
	status, _ := reader.ReadString('\n')
	if !strings.Contains(status, "101") {
		t.Fatalf("status: %s", status)
	}
	for {
		line, _ := reader.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}

	server.Events.Publish(Event{Event: "plan.updated", Data: map[string]any{"plan_id": "p"}}, "")
	payload, err := readFrame(reader)
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	var ev Event
	if err := json.Unmarshal(payload, &ev); err != nil {
		t.Fatalf("json: %v", err)
	}
	if ev.Event != "plan.updated" {
		t.Fatalf("event: %s", ev.Event)
	}
}

func TestHandleWSContextDone(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	writer, client := newPipeWSWriter()
	defer client.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req = req.WithContext(context.Background())
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	req = req.WithContext(ctx)
	go func() {
		server.handleWS(writer, req)
		close(done)
	}()
	buf := bufio.NewReader(client)
	_, _ = buf.ReadString('\n')
	for {
		line, _ := buf.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	cancel()
	<-done
}

func TestHandleWSMarshalError(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	writer, client := newPipeWSWriter()
	defer client.Close()
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil).WithContext(ctx)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	done := make(chan struct{})
	go func() {
		server.handleWS(writer, req)
		close(done)
	}()
	buf := bufio.NewReader(client)
	_, _ = buf.ReadString('\n')
	for {
		line, _ := buf.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	waitForSub(t, server.Events)
	server.Events.Publish(Event{Event: "bad", Data: make(chan int)}, "")
	cancel()
	<-done
}

func TestHandleWSClosedChannel(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	writer, client := newPipeWSWriter()
	defer client.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	done := make(chan struct{})
	go func() {
		server.handleWS(writer, req)
		close(done)
	}()
	buf := bufio.NewReader(client)
	_, _ = buf.ReadString('\n')
	for {
		line, _ := buf.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		server.Events.mu.Lock()
		var channels []chan Event
		for sessionID, subs := range server.Events.subs {
			for id, ch := range subs {
				delete(subs, id)
				channels = append(channels, ch)
			}
			delete(server.Events.subs, sessionID)
		}
		server.Events.mu.Unlock()
		for _, ch := range channels {
			close(ch)
		}
		if len(channels) > 0 {
			<-done
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestHandleWSWriteError(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	writer, client := newPipeWSWriter()
	defer client.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	done := make(chan struct{})
	oldWrite := writeWSFrame
	writeWSFrame = func(_ *bufio.ReadWriter, _ []byte) error { return fmt.Errorf("boom") }
	defer func() { writeWSFrame = oldWrite }()
	go func() {
		server.handleWS(writer, req)
		close(done)
	}()
	buf := bufio.NewReader(client)
	_, _ = buf.ReadString('\n')
	for {
		line, _ := buf.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	waitForSub(t, server.Events)
	server.Events.Publish(Event{Event: "plan.updated", Data: map[string]any{"plan_id": "p"}}, "")
	<-done
}

func TestHandleWSFlushError(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	writer, client := newPipeWSWriter()
	defer client.Close()
	req := httptest.NewRequest(http.MethodGet, "/v1/ws", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Key", "key")
	done := make(chan struct{})
	oldFlush := flushWS
	flushWS = func(_ *bufio.ReadWriter) error { return fmt.Errorf("flush") }
	defer func() { flushWS = oldFlush }()
	go func() {
		server.handleWS(writer, req)
		close(done)
	}()
	buf := bufio.NewReader(client)
	_, _ = buf.ReadString('\n')
	for {
		line, _ := buf.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	waitForSub(t, server.Events)
	server.Events.Publish(Event{Event: "plan.updated", Data: map[string]any{"plan_id": "p"}}, "")
	<-done
}

func readFrame(r *bufio.Reader) ([]byte, error) {
	b1, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if b1 != 0x81 {
		return nil, fmt.Errorf("opcode: %x", b1)
	}
	b2, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	length := int(b2 & 0x7f)
	switch length {
	case 126:
		b1, _ = r.ReadByte()
		b2, _ = r.ReadByte()
		length = int(b1)<<8 | int(b2)
	case 127:
		var n uint64
		for i := 0; i < 8; i++ {
			b, _ := r.ReadByte()
			n = (n << 8) | uint64(b)
		}
		length = int(n)
	}
	out := make([]byte, length)
	_, err = io.ReadFull(r, out)
	return out, err
}

func TestHandleExecutionLogsMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/executions/exec/logs", nil)
	w := httptest.NewRecorder()
	srv := &Server{}
	srv.handleExecutionLogs(w, req, "exec")
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionLogsMissingExecution(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/executions//logs", nil)
	w := httptest.NewRecorder()
	srv := &Server{}
	srv.handleExecutionLogs(w, req, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleExecutionLogsNoFlusher(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs", nil)
	w := &noFlushWriter{}
	srv := &Server{Logs: NewLogHub()}
	srv.handleExecutionLogs(w, req, "exec")
	if w.status != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.status)
	}
}

func TestHandleExecutionLogsStream(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs?level=info", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()

	reader := bufio.NewReader(pr)
	_, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ok: %v", err)
	}

	server.Logs.Append(LogLine{ExecutionID: "exec", Level: "info", Message: "hello", Timestamp: time.Now().UTC()})
	var dataLine string
	deadline := time.After(500 * time.Millisecond)
	for dataLine == "" {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimPrefix(line, "data: ")
			dataLine = strings.TrimSpace(dataLine)
		}
	}
	var line LogLine
	if err := json.Unmarshal([]byte(dataLine), &line); err != nil {
		t.Fatalf("json: %v", err)
	}
	if line.Message != "hello" {
		t.Fatalf("message: %s", line.Message)
	}
	cancel()
	<-done
}

func TestHandleEventsSSEMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/events", nil)
	w := httptest.NewRecorder()
	srv := &Server{}
	srv.handleEventsSSE(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status: %d", w.Code)
	}
}

func TestHandleEventsSSENoFlusher(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil)
	w := &noFlushWriter{}
	srv := &Server{Events: NewEventHub()}
	srv.handleEventsSSE(w, req)
	if w.status != http.StatusInternalServerError {
		t.Fatalf("status: %d", w.status)
	}
}

func TestHandleEventsSSEStream(t *testing.T) {
	server := &Server{Events: NewEventHub()}
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/events", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		server.handleEventsSSE(writer, req)
		_ = pw.Close()
		close(done)
	}()

	reader := bufio.NewReader(pr)
	_, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read ok: %v", err)
	}

	waitForSub(t, server.Events)
	server.Events.Publish(Event{Event: "plan.updated", Data: map[string]any{"plan_id": "p"}}, "")

	var dataLine string
	deadline := time.After(500 * time.Millisecond)
	for dataLine == "" {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		if strings.HasPrefix(line, "data: ") {
			dataLine = strings.TrimSpace(strings.TrimPrefix(line, "data: "))
		}
	}
	var ev Event
	if err := json.Unmarshal([]byte(dataLine), &ev); err != nil {
		t.Fatalf("json: %v", err)
	}
	if ev.Event != "plan.updated" {
		t.Fatalf("event: %s", ev.Event)
	}
	cancel()
	<-done
}

func TestHandleExecutionLogsFilters(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	server.Logs.Append(LogLine{ExecutionID: "exec", ToolCallID: "tool2", Level: "info", Message: "skip"})
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs?tool_call_id=tool1&level=error", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()
	reader := bufio.NewReader(pr)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(line, ":ok") {
		t.Fatalf("unexpected line: %s", line)
	}
	cancel()
	<-done
}

func TestHandleExecutionLogsLevelFilterMismatch(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	server.Logs.Append(LogLine{ExecutionID: "exec", Level: "info", Message: "skip"})
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs?level=error", nil).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()
	reader := bufio.NewReader(pr)
	line, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.HasPrefix(line, ":ok") {
		t.Fatalf("unexpected line: %s", line)
	}
	cancel()
	<-done
}

func TestHandleExecutionLogsMarshalError(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs", nil).WithContext(ctx)
	done := make(chan struct{})
	oldMarshal := marshalLogJSON
	marshalLogJSON = func(v any) ([]byte, error) { return nil, fmt.Errorf("boom") }
	defer func() { marshalLogJSON = oldMarshal }()
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()
	reader := bufio.NewReader(pr)
	_, _ = reader.ReadString('\n')
	waitForLogSub(t, server.Logs, "exec")
	server.Logs.Append(LogLine{ExecutionID: "exec", Level: "info", Message: "hello"})
	cancel()
	<-done
}

func TestHandleExecutionLogsHistoryMarshalError(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	server.Logs.Append(LogLine{ExecutionID: "exec", Level: "info", Message: "history"})
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs", nil)
	done := make(chan struct{})
	oldMarshal := marshalLogJSON
	marshalLogJSON = func(v any) ([]byte, error) { return nil, fmt.Errorf("boom") }
	defer func() { marshalLogJSON = oldMarshal }()
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()
	reader := bufio.NewReader(pr)
	_, _ = reader.ReadString('\n')
	<-done
}

func TestHandleExecutionLogsChannelClosed(t *testing.T) {
	server := &Server{Logs: NewLogHub()}
	pr, pw := io.Pipe()
	writer := &pipeWriter{w: pw}
	req := httptest.NewRequest(http.MethodGet, "/v1/executions/exec/logs", nil)
	done := make(chan struct{})
	go func() {
		server.handleExecutionByID(writer, req)
		_ = pw.Close()
		close(done)
	}()
	reader := bufio.NewReader(pr)
	_, _ = reader.ReadString('\n')
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		server.Logs.mu.Lock()
		subs := server.Logs.subs["exec"]
		for id, ch := range subs {
			delete(subs, id)
			server.Logs.mu.Unlock()
			close(ch)
			<-done
			return
		}
		server.Logs.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
}

type pipeWriter struct {
	header http.Header
	status int
	w      *io.PipeWriter
}

func (w *pipeWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *pipeWriter) Write(p []byte) (int, error) {
	return w.w.Write(p)
}

func (w *pipeWriter) WriteHeader(status int) {
	w.status = status
}

func (w *pipeWriter) Flush() {}

func waitForSub(t *testing.T, hub *EventHub) {
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		hub.mu.Lock()
		if len(hub.subs) > 0 {
			hub.mu.Unlock()
			return
		}
		hub.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
}

func waitForLogSub(t *testing.T, hub *LogHub, execID string) {
	deadline := time.After(200 * time.Millisecond)
	for {
		select {
		case <-deadline:
			t.Fatalf("timeout")
		default:
		}
		hub.mu.Lock()
		if len(hub.subs[execID]) > 0 {
			hub.mu.Unlock()
			return
		}
		hub.mu.Unlock()
		time.Sleep(5 * time.Millisecond)
	}
}
