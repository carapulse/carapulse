package web

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"
)

const wsGUID = "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"

var writeWSFrame = writeTextFrame
var flushWS = func(buf *bufio.ReadWriter) error { return buf.Flush() }

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := sessionIDFromRequest(r)
	if s != nil && s.SessionRequired {
		id, err := s.requireSession(r)
		if err != nil {
			http.Error(w, "session denied", http.StatusForbidden)
			return
		}
		sessionID = id
	}
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijack unsupported", http.StatusInternalServerError)
		return
	}
	conn, buf, err := upgradeWebSocket(r, hijacker)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close()

	hub := s.eventHub()
	sub, cancel := hub.Subscribe(sessionID)
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case ev, ok := <-sub:
			if !ok {
				return
			}
			ev.TS = time.Now().UTC()
			if ev.SessionID == "" {
				ev.SessionID = sessionID
			}
			payload, err := json.Marshal(ev)
			if err != nil {
				continue
			}
			if err := writeWSFrame(buf, payload); err != nil {
				return
			}
			if err := flushWS(buf); err != nil {
				return
			}
		}
	}
}

func upgradeWebSocket(r *http.Request, hijacker http.Hijacker) (net.Conn, *bufio.ReadWriter, error) {
	if !isWebSocketRequest(r) {
		return nil, nil, errors.New("not websocket")
	}
	key := r.Header.Get("Sec-WebSocket-Key")
	if key == "" {
		return nil, nil, errors.New("missing key")
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		return nil, nil, err
	}
	accept := computeAccept(key)
	_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	_, _ = buf.WriteString("Upgrade: websocket\r\n")
	_, _ = buf.WriteString("Connection: Upgrade\r\n")
	_, _ = buf.WriteString("Sec-WebSocket-Accept: " + accept + "\r\n\r\n")
	if err := buf.Flush(); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	return conn, buf, nil
}

func isWebSocketRequest(r *http.Request) bool {
	if !headerHasToken(r.Header, "Connection", "upgrade") {
		return false
	}
	if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
		return false
	}
	return true
}

func headerHasToken(h http.Header, key, token string) bool {
	for _, v := range h.Values(key) {
		for _, part := range strings.Split(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), token) {
				return true
			}
		}
	}
	return false
}

func computeAccept(key string) string {
	sum := sha1.Sum([]byte(key + wsGUID))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func writeTextFrame(w *bufio.ReadWriter, payload []byte) error {
	header := []byte{0x81}
	n := len(payload)
	switch {
	case n <= 125:
		header = append(header, byte(n))
	case n <= 65535:
		header = append(header, 126, byte(n>>8), byte(n))
	default:
		header = append(header, 127,
			byte(n>>56), byte(n>>48), byte(n>>40), byte(n>>32),
			byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
	if _, err := w.Write(header); err != nil {
		return err
	}
	_, err := w.Write(payload)
	return err
}
