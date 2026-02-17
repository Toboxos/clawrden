package warden

import (
	"clawrden/pkg/protocol"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Decision represents a human reviewer's decision.
type Decision int

const (
	DecisionApprove Decision = iota
	DecisionDeny
)

// PendingRequest holds a command awaiting human approval.
type PendingRequest struct {
	ID        string            `json:"id"`
	Request   *protocol.Request `json:"request"`
	Timestamp time.Time         `json:"timestamp"`
	decision  chan Decision
}

// HITLQueue manages pending requests awaiting human approval.
type HITLQueue struct {
	mu       sync.RWMutex
	pending  map[string]*PendingRequest
	counter  atomic.Int64
}

// NewHITLQueue creates a new HITL approval queue.
func NewHITLQueue() *HITLQueue {
	return &HITLQueue{
		pending: make(map[string]*PendingRequest),
	}
}

// Enqueue adds a request to the pending queue and blocks until a decision is made
// or the context is cancelled. Returns the decision.
func (q *HITLQueue) Enqueue(ctx context.Context, req *protocol.Request) Decision {
	id := q.nextID()
	pr := &PendingRequest{
		ID:        id,
		Request:   req,
		Timestamp: time.Now(),
		decision:  make(chan Decision, 1),
	}

	q.mu.Lock()
	q.pending[id] = pr
	q.mu.Unlock()

	defer func() {
		q.mu.Lock()
		delete(q.pending, id)
		q.mu.Unlock()
	}()

	select {
	case d := <-pr.decision:
		return d
	case <-ctx.Done():
		return DecisionDeny
	}
}

// Resolve resolves a pending request with the given decision.
func (q *HITLQueue) Resolve(id string, decision Decision) bool {
	q.mu.RLock()
	pr, ok := q.pending[id]
	q.mu.RUnlock()

	if !ok {
		return false
	}

	select {
	case pr.decision <- decision:
		return true
	default:
		return false // Already resolved
	}
}

// List returns all currently pending requests.
func (q *HITLQueue) List() []PendingRequest {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]PendingRequest, 0, len(q.pending))
	for _, pr := range q.pending {
		result = append(result, PendingRequest{
			ID:        pr.ID,
			Request:   pr.Request,
			Timestamp: pr.Timestamp,
		})
	}
	return result
}

// nextID generates a unique request ID.
func (q *HITLQueue) nextID() string {
	n := q.counter.Add(1)
	return "req-" + time.Now().Format("20060102-150405") + "-" + itoa(n)
}

// itoa is a simple int-to-string without importing strconv.
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[pos:])
}
