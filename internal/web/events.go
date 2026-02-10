package web

import (
	"sync"
	"time"
)

type Event struct {
	Event     string    `json:"event"`
	SessionID string    `json:"session_id,omitempty"`
	Data      any       `json:"data"`
	TS        time.Time `json:"ts"`
}

type EventHub struct {
	mu     sync.Mutex
	nextID int
	subs   map[string]map[int]chan Event
}

func NewEventHub() *EventHub {
	return &EventHub{subs: make(map[string]map[int]chan Event)}
}

func (h *EventHub) Subscribe(sessionID string) (<-chan Event, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = make(map[string]map[int]chan Event)
	}
	if h.subs[sessionID] == nil {
		h.subs[sessionID] = make(map[int]chan Event)
	}
	id := h.nextID
	h.nextID++
	ch := make(chan Event, 8)
	h.subs[sessionID][id] = ch
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.subs == nil {
			return
		}
		if subs, ok := h.subs[sessionID]; ok {
			if sub, ok := subs[id]; ok {
				delete(subs, id)
				close(sub)
			}
			if len(subs) == 0 {
				delete(h.subs, sessionID)
			}
		}
	}
	return ch, cancel
}

func (h *EventHub) Publish(ev Event, sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		return
	}
	if subs, ok := h.subs[sessionID]; ok {
		for _, ch := range subs {
			select {
			case ch <- ev:
			default:
			}
		}
	}
	if sessionID != "" {
		if subs, ok := h.subs[""]; ok {
			for _, ch := range subs {
				select {
				case ch <- ev:
				default:
				}
			}
		}
	}
}

type LogLine struct {
	ExecutionID string    `json:"execution_id"`
	ToolCallID  string    `json:"tool_call_id,omitempty"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	Timestamp   time.Time `json:"timestamp"`
}

type LogHub struct {
	mu       sync.Mutex
	nextID   int
	subs     map[string]map[int]chan LogLine
	history  map[string][]LogLine
	maxLines int
	maxKeys  int
}

func NewLogHub() *LogHub {
	return &LogHub{
		subs:     make(map[string]map[int]chan LogLine),
		history:  make(map[string][]LogLine),
		maxLines: 200,
		maxKeys:  500,
	}
}

func (h *LogHub) Subscribe(executionID string) (<-chan LogLine, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = make(map[string]map[int]chan LogLine)
	}
	if h.subs[executionID] == nil {
		h.subs[executionID] = make(map[int]chan LogLine)
	}
	id := h.nextID
	h.nextID++
	ch := make(chan LogLine, 8)
	h.subs[executionID][id] = ch
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.subs == nil {
			return
		}
		if subs, ok := h.subs[executionID]; ok {
			if sub, ok := subs[id]; ok {
				delete(subs, id)
				close(sub)
			}
			if len(subs) == 0 {
				delete(h.subs, executionID)
			}
		}
	}
	return ch, cancel
}

func (h *LogHub) Append(line LogLine) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.history == nil {
		h.history = make(map[string][]LogLine)
	}
	history := append(h.history[line.ExecutionID], line)
	if len(history) > h.maxLines {
		history = history[len(history)-h.maxLines:]
	}
	h.history[line.ExecutionID] = history
	// Evict oldest history entries when we have too many keys to prevent
	// unbounded memory growth over the lifetime of the process.
	if h.maxKeys > 0 && len(h.history) > h.maxKeys {
		for key := range h.history {
			if key == line.ExecutionID {
				continue
			}
			if _, hasSub := h.subs[key]; !hasSub {
				delete(h.history, key)
			}
			if len(h.history) <= h.maxKeys {
				break
			}
		}
	}
	for _, ch := range h.subs[line.ExecutionID] {
		select {
		case ch <- line:
		default:
		}
	}
}

func (h *LogHub) History(executionID string) []LogLine {
	h.mu.Lock()
	defer h.mu.Unlock()
	history := h.history[executionID]
	out := make([]LogLine, len(history))
	copy(out, history)
	return out
}
