package bridge

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventDispatcherSerializesConcurrentProducers(t *testing.T) {
	const (
		producerCount     = 32
		eventsPerProducer = 64
		totalEvents       = producerCount * eventsPerProducer
	)

	var activeDeliveries atomic.Int32
	var maximumDeliveries atomic.Int32
	var delivered atomic.Int32
	allDelivered := make(chan struct{})
	var deliveredOnce sync.Once

	sink := func(_ context.Context, _ string, _ ...any) {
		active := activeDeliveries.Add(1)
		for {
			maximum := maximumDeliveries.Load()
			if active <= maximum ||
				maximumDeliveries.CompareAndSwap(maximum, active) {
				break
			}
		}

		// Increase the chance that concurrent sink calls would overlap if the
		// dispatcher did not serialize them.
		time.Sleep(20 * time.Microsecond)

		activeDeliveries.Add(-1)
		if delivered.Add(1) == totalEvents {
			deliveredOnce.Do(func() {
				close(allDelivered)
			})
		}
	}

	dispatcher := newEventDispatcher(
		context.Background(),
		sink,
		totalEvents,
	)
	defer dispatcher.stop(time.Second)

	var producers sync.WaitGroup
	producers.Add(producerCount)
	for producer := 0; producer < producerCount; producer++ {
		go func(producer int) {
			defer producers.Done()
			for sequence := 0; sequence < eventsPerProducer; sequence++ {
				if !dispatcher.enqueue(eventEnvelope{
					name: "test",
					data: [2]int{producer, sequence},
				}) {
					t.Errorf("enqueue rejected while dispatcher was active")
					return
				}
			}
		}(producer)
	}
	producers.Wait()

	select {
	case <-allDelivered:
	case <-time.After(5 * time.Second):
		t.Fatalf(
			"timed out after %d of %d events",
			delivered.Load(),
			totalEvents,
		)
	}

	if maximum := maximumDeliveries.Load(); maximum != 1 {
		t.Fatalf("maximum concurrent sink calls = %d, want 1", maximum)
	}
}

func TestEventDispatcherPreservesSequentialOrder(t *testing.T) {
	const eventCount = 128

	delivered := make(chan int, eventCount)
	dispatcher := newEventDispatcher(
		context.Background(),
		func(_ context.Context, _ string, data ...any) {
			delivered <- data[0].(int)
		},
		eventCount,
	)
	defer dispatcher.stop(time.Second)

	for sequence := 0; sequence < eventCount; sequence++ {
		if !dispatcher.enqueue(eventEnvelope{
			name: "ordered",
			data: sequence,
		}) {
			t.Fatalf("enqueue %d rejected", sequence)
		}
	}

	for expected := 0; expected < eventCount; expected++ {
		select {
		case actual := <-delivered:
			if actual != expected {
				t.Fatalf(
					"event %d delivered out of order as %d",
					expected,
					actual,
				)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for event %d", expected)
		}
	}
}

func TestEventDispatcherRecoversSinkPanic(t *testing.T) {
	var calls atomic.Int32
	deliveredAfterPanic := make(chan struct{})

	dispatcher := newEventDispatcher(
		context.Background(),
		func(_ context.Context, _ string, _ ...any) {
			if calls.Add(1) == 1 {
				panic("synthetic sink failure")
			}
			close(deliveredAfterPanic)
		},
		2,
	)
	defer dispatcher.stop(time.Second)

	if !dispatcher.enqueue(eventEnvelope{name: "first"}) {
		t.Fatal("first enqueue rejected")
	}
	if !dispatcher.enqueue(eventEnvelope{name: "second"}) {
		t.Fatal("second enqueue rejected")
	}

	select {
	case <-deliveredAfterPanic:
	case <-time.After(time.Second):
		t.Fatal("dispatcher stopped after a recovered sink panic")
	}
}

func TestEventDispatcherStopUnblocksBlockedProducer(t *testing.T) {
	sinkStarted := make(chan struct{})
	releaseSink := make(chan struct{})
	dispatcher := newEventDispatcher(
		context.Background(),
		func(_ context.Context, _ string, _ ...any) {
			select {
			case <-sinkStarted:
			default:
				close(sinkStarted)
			}
			<-releaseSink
		},
		1,
	)

	if !dispatcher.enqueue(eventEnvelope{name: "in-flight"}) {
		t.Fatal("in-flight enqueue rejected")
	}
	<-sinkStarted
	if !dispatcher.enqueue(eventEnvelope{name: "queued"}) {
		t.Fatal("queued enqueue rejected")
	}

	blockedResult := make(chan bool, 1)
	go func() {
		blockedResult <- dispatcher.enqueue(eventEnvelope{name: "blocked"})
	}()

	// Give the producer an opportunity to block on the full queue.
	time.Sleep(10 * time.Millisecond)
	stopped := make(chan bool, 1)
	go func() {
		stopped <- dispatcher.stop(50 * time.Millisecond)
	}()

	select {
	case accepted := <-blockedResult:
		if accepted {
			t.Fatal("blocked enqueue succeeded after dispatcher stopped")
		}
	case <-time.After(time.Second):
		t.Fatal("blocked producer was not released by dispatcher shutdown")
	}

	if stoppedCleanly := <-stopped; stoppedCleanly {
		t.Fatal("stop unexpectedly completed while the sink was blocked")
	}

	close(releaseSink)
	select {
	case <-dispatcher.done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher worker did not exit after sink was released")
	}
}
