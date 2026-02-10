package tools

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type LogLine struct {
	ToolCallID  string    `json:"tool_call_id"`
	ExecutionID string    `json:"execution_id,omitempty"`
	Tool        string    `json:"tool"`
	Action      string    `json:"action"`
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

func (h *LogHub) Subscribe(toolCallID string) (<-chan LogLine, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		h.subs = make(map[string]map[int]chan LogLine)
	}
	if h.subs[toolCallID] == nil {
		h.subs[toolCallID] = make(map[int]chan LogLine)
	}
	id := h.nextID
	h.nextID++
	ch := make(chan LogLine, 8)
	h.subs[toolCallID][id] = ch
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.subs == nil {
			return
		}
		if subs, ok := h.subs[toolCallID]; ok {
			if sub, ok := subs[id]; ok {
				delete(subs, id)
				close(sub)
			}
			if len(subs) == 0 {
				delete(h.subs, toolCallID)
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
	history := append(h.history[line.ToolCallID], line)
	if h.maxLines > 0 && len(history) > h.maxLines {
		history = history[len(history)-h.maxLines:]
	}
	h.history[line.ToolCallID] = history
	// Evict oldest history entries when we have too many keys to prevent
	// unbounded memory growth over the lifetime of the process.
	if h.maxKeys > 0 && len(h.history) > h.maxKeys {
		for key := range h.history {
			if key == line.ToolCallID {
				continue
			}
			// Only evict keys with no active subscribers.
			if _, hasSub := h.subs[key]; !hasSub {
				delete(h.history, key)
			}
			if len(h.history) <= h.maxKeys {
				break
			}
		}
	}
	for _, ch := range h.subs[line.ToolCallID] {
		select {
		case ch <- line:
		default:
		}
	}
}

func (h *LogHub) History(toolCallID string) []LogLine {
	h.mu.Lock()
	defer h.mu.Unlock()
	history := h.history[toolCallID]
	out := make([]LogLine, len(history))
	copy(out, history)
	return out
}

var toolCallCounter uint64

func newToolCallID() string {
	return fmt.Sprintf("tool_%d", atomic.AddUint64(&toolCallCounter, 1))
}

func truncateMessage(msg string) string {
	const max = 4096
	msg = strings.TrimSpace(msg)
	if len(msg) <= max {
		return msg
	}
	return msg[:max] + "..."
}
