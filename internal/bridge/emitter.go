package bridge

import (
	"context"
	"log"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const (
	eventQueueCapacity    = 4096
	dispatcherStopTimeout = 2 * time.Second
)

type eventEnvelope struct {
	name string
	data any
}

type eventSink func(context.Context, string, ...any)

// eventDispatcher is the only component allowed to call the Wails event sink.
// Producers may enqueue concurrently, but delivery and JSON serialization are
// always performed sequentially by the worker goroutine.
type eventDispatcher struct {
	ctx    context.Context
	cancel context.CancelFunc
	sink   eventSink
	queue  chan eventEnvelope
	done   chan struct{}
	active atomic.Bool
}

func newEventDispatcher(
	parent context.Context,
	sink eventSink,
	queueCapacity int,
) *eventDispatcher {
	if parent == nil {
		parent = context.Background()
	}
	if sink == nil {
		panic("bridge: event sink must not be nil")
	}
	if queueCapacity <= 0 {
		panic("bridge: event queue capacity must be positive")
	}

	dispatcherContext, cancel := context.WithCancel(parent)
	dispatcher := &eventDispatcher{
		ctx:    dispatcherContext,
		cancel: cancel,
		sink:   sink,
		queue:  make(chan eventEnvelope, queueCapacity),
		done:   make(chan struct{}),
	}
	dispatcher.active.Store(true)
	go dispatcher.run()
	return dispatcher
}

func (dispatcher *eventDispatcher) run() {
	defer close(dispatcher.done)

	for {
		select {
		case <-dispatcher.ctx.Done():
			return
		case event := <-dispatcher.queue:
			dispatcher.deliver(event)
		}
	}
}

func (dispatcher *eventDispatcher) deliver(event eventEnvelope) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// Do not emit this failure as another Wails event: doing so would
			// recurse through the component that just panicked.
			log.Printf(
				"Recovered panic while delivering Wails event %q: %v\n%s",
				event.name,
				recovered,
				debug.Stack(),
			)
		}
	}()

	dispatcher.sink(dispatcher.ctx, event.name, event.data)
}

func (dispatcher *eventDispatcher) enqueue(event eventEnvelope) bool {
	if !dispatcher.active.Load() {
		return false
	}

	// A bounded queue prevents unbounded memory growth. Backpressure is
	// preferable to silently dropping request/response summaries because a
	// dropped summary would leave Traffic Inspector inconsistent with history.
	select {
	case dispatcher.queue <- event:
		return true
	case <-dispatcher.ctx.Done():
		return false
	}
}

func (dispatcher *eventDispatcher) stop(timeout time.Duration) bool {
	dispatcher.active.Store(false)
	dispatcher.cancel()

	if timeout <= 0 {
		<-dispatcher.done
		return true
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-dispatcher.done:
		return true
	case <-timer.C:
		return false
	}
}

var (
	dispatcherMu sync.RWMutex
	dispatcher   *eventDispatcher
)

func Init(ctx context.Context) {
	nextDispatcher := newEventDispatcher(
		ctx,
		runtime.EventsEmit,
		eventQueueCapacity,
	)

	dispatcherMu.Lock()
	previousDispatcher := dispatcher
	dispatcher = nextDispatcher
	dispatcherMu.Unlock()

	if previousDispatcher != nil &&
		!previousDispatcher.stop(dispatcherStopTimeout) {
		log.Printf("Timed out while stopping the previous Wails event dispatcher")
	}
}

func Shutdown() {
	dispatcherMu.Lock()
	currentDispatcher := dispatcher
	dispatcher = nil
	dispatcherMu.Unlock()

	if currentDispatcher != nil &&
		!currentDispatcher.stop(dispatcherStopTimeout) {
		log.Printf("Timed out while stopping the Wails event dispatcher")
	}
}

func emit(event string, data any) {
	dispatcherMu.RLock()
	currentDispatcher := dispatcher
	dispatcherMu.RUnlock()

	if currentDispatcher == nil {
		return
	}
	currentDispatcher.enqueue(eventEnvelope{name: event, data: data})
}

func EmitError(m string) {
	emit("error", m)
	log.Println(m)
}

func EmitProxyStopped() {
	emit("proxy-stopped", nil)
}

type HTTPRequestSummary struct {
	ID               uint64               `json:"id"`
	Host             string               `json:"host"`
	Port             string               `json:"port"`
	Version          string               `json:"version"`
	Method           string               `json:"method"`
	Path             string               `json:"path"`
	Scheme           string               `json:"scheme"` // "http" o "https"
	ReceivedAtMs     int64                `json:"receivedAtMs"`
	FilterMatches    []HistoryFilterMatch `json:"filterMatches"`
	EvaluatedFilters []HistoryFilterMatch `json:"evaluatedFilters"`
}

type HTTPResponseSummary struct {
	ID                          uint64               `json:"id"`
	Host                        string               `json:"host"`
	Port                        string               `json:"port"`
	Version                     string               `json:"version"`
	StatusCode                  int                  `json:"statusCode"`
	ReceivedAtMs                int64                `json:"receivedAtMs"`
	FilterMatches               []HistoryFilterMatch `json:"filterMatches"`
	EvaluatedFilters            []HistoryFilterMatch `json:"evaluatedFilters"`
	UnsupportedContentEncodings []string             `json:"unsupportedContentEncodings"`
	ContentDecodingFailed       bool                 `json:"contentDecodingFailed"`
}

func EmitHTTPRequestSummary(request HTTPRequestSummary) {
	request.FilterMatches, request.EvaluatedFilters =
		getActiveFilterEvaluationForEntryID(request.ID)
	emit("request", request)
}

func EmitHTTPResponseSummary(response HTTPResponseSummary) {
	response.FilterMatches, response.EvaluatedFilters =
		getActiveFilterEvaluationForEntryID(response.ID)
	emit("response", response)
}

func getActiveFilterEvaluationForEntryID(
	entryID uint64,
) ([]HistoryFilterMatch, []HistoryFilterMatch) {
	historyMu.RLock()
	defer historyMu.RUnlock()

	activeFilters := snapshotActiveHistoryFilters()
	if len(activeFilters) == 0 {
		return []HistoryFilterMatch{}, []HistoryFilterMatch{}
	}

	evaluatedFilters := make([]HistoryFilterMatch, 0, len(activeFilters))
	for _, filter := range activeFilters {
		evaluatedFilters = append(evaluatedFilters, HistoryFilterMatch{
			FilterID: filter.filterID,
			Version:  filter.version,
		})
	}

	entry, ok := history[entryID]
	if !ok || entry == nil {
		return []HistoryFilterMatch{}, evaluatedFilters
	}

	candidateCache := make(map[HistoryFilterTarget]string, 8)
	matches := make([]HistoryFilterMatch, 0, len(activeFilters))

	for _, filter := range activeFilters {
		if entryMatchesCompiledHistoryFilters(entry, filter, candidateCache) {
			matches = append(matches, HistoryFilterMatch{
				FilterID: filter.filterID,
				Version:  filter.version,
			})
		}
	}

	return matches, evaluatedFilters
}
