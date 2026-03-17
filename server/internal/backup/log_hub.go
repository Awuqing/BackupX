package backup

import (
	"sync"
	"time"
)

type LogHub struct {
	mu      sync.RWMutex
	streams map[uint]*logStreamState
}

type logStreamState struct {
	nextSequence int64
	events       []LogEvent
	subscribers  map[int]chan LogEvent
	nextSubID    int
	completed    bool
	status       string
}

func NewLogHub() *LogHub {
	return &LogHub{streams: make(map[uint]*logStreamState)}
}

func (h *LogHub) Append(recordID uint, level, message string) LogEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.ensureState(recordID)
	state.nextSequence++
	event := LogEvent{RecordID: recordID, Sequence: state.nextSequence, Level: level, Message: message, Timestamp: time.Now().UTC(), Status: state.status}
	state.events = append(state.events, event)
	for _, subscriber := range state.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
	return event
}

func (h *LogHub) Snapshot(recordID uint) []LogEvent {
	h.mu.RLock()
	defer h.mu.RUnlock()
	state, ok := h.streams[recordID]
	if !ok {
		return nil
	}
	result := make([]LogEvent, len(state.events))
	copy(result, state.events)
	return result
}

func (h *LogHub) Subscribe(recordID uint, buffer int) (<-chan LogEvent, func()) {
	if buffer <= 0 {
		buffer = 32
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.ensureState(recordID)
	state.nextSubID++
	id := state.nextSubID
	channel := make(chan LogEvent, buffer)
	state.subscribers[id] = channel
	for _, event := range state.events {
		channel <- event
	}
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		stream, ok := h.streams[recordID]
		if !ok {
			return
		}
		subscriber, ok := stream.subscribers[id]
		if !ok {
			return
		}
		delete(stream.subscribers, id)
		close(subscriber)
	}
	return channel, cancel
}

func (h *LogHub) Complete(recordID uint, status string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	state := h.ensureState(recordID)
	state.completed = true
	state.status = status
	state.nextSequence++
	event := LogEvent{RecordID: recordID, Sequence: state.nextSequence, Level: "info", Message: "stream completed", Timestamp: time.Now().UTC(), Completed: true, Status: status}
	state.events = append(state.events, event)
	for _, subscriber := range state.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func (h *LogHub) ensureState(recordID uint) *logStreamState {
	state, ok := h.streams[recordID]
	if ok {
		return state
	}
	state = &logStreamState{subscribers: make(map[int]chan LogEvent), status: "running"}
	h.streams[recordID] = state
	return state
}
