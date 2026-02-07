package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

var marshalLogJSON = json.Marshal

func (s *Server) handleExecutionLogs(w http.ResponseWriter, r *http.Request, executionID string) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if executionID == "" {
		http.Error(w, "missing execution", http.StatusBadRequest)
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

	toolFilter := strings.TrimSpace(r.URL.Query().Get("tool_call_id"))
	levelFilter := strings.TrimSpace(r.URL.Query().Get("level"))

	writeLine := func(line LogLine) bool {
		if toolFilter != "" && line.ToolCallID != toolFilter {
			return true
		}
		if levelFilter != "" && line.Level != levelFilter {
			return true
		}
		payload, err := marshalLogJSON(line)
		if err != nil {
			return false
		}
		_, _ = fmt.Fprintf(w, "event: log\ndata: %s\n\n", payload)
		flusher.Flush()
		return true
	}

	hub := s.logHub()
	for _, line := range hub.History(executionID) {
		if ok := writeLine(line); !ok {
			return
		}
	}

	ch, cancel := hub.Subscribe(executionID)
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
