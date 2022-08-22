package mem

import (
	"context"
	"sync"
	"time"
)

type storeGCWorker struct {
	gcCh chan func()
	stop func()
	wg   sync.WaitGroup
}

func (gc *storeGCWorker) Start(freq time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())

	*gc = storeGCWorker{
		gcCh: make(chan func()),
		stop: cancel,
	}

	gc.wg.Add(1)
	go func() {
		defer gc.wg.Done()
		defer cancel()

		ticker := time.NewTicker(freq)
		defer ticker.Stop()

		var gcs []func()
		for {
			select {
			case <-ctx.Done():
				return
			case gc := <-gc.gcCh:
				gcs = append(gcs, gc)
			case <-ticker.C:
				for _, gc := range gcs {
					gc()
				}
			}
		}
	}()
}

func (gc *storeGCWorker) Close() {
	gc.stop()
	gc.wg.Wait()
}

func (gc *storeGCWorker) Add(gcFunc func()) {
	gc.gcCh <- gcFunc
}

type inMemoryStore[K comparable, V any] struct {
	mut     sync.RWMutex
	entries map[K]inMemoryValue[V]
	stopGC  func()
	maxAge  time.Duration
}

func (s *inMemoryStore[K, V]) init(maxAge time.Duration) {
	*s = inMemoryStore[K, V]{
		entries: make(map[K]inMemoryValue[V], 25),
		stopGC:  func() {},
		maxAge:  maxAge,
	}
}

func (s *inMemoryStore[K, V]) Close() {
	s.mut.Lock()
	defer s.mut.Unlock()

	if s.stopGC != nil {
		s.stopGC()
	}

	s.entries = nil
}

func (s *inMemoryStore[K, V]) startGC() {
	s.mut.Lock()
	defer s.mut.Unlock()

	var gc storeGCWorker
	gc.Start(s.maxAge)
	gc.Add(s.doGC)

	s.stopGC = gc.Close
}

func (s *inMemoryStore[K, V]) doGC() {
	s.mut.Lock()
	defer s.mut.Unlock()

	for k, entry := range s.entries {
		if entry.isExpired() {
			delete(s.entries, k)
		}
	}
}

func (s *inMemoryStore[K, V]) Get(key K) (V, bool) {
	s.mut.RLock()
	defer s.mut.RUnlock()

	e, ok := s.entries[key]
	return e.value, ok && !e.isExpired()
}

func (s *inMemoryStore[K, V]) Set(key K, value V) {
	var now time.Time
	if s.maxAge > 0 {
		now = time.Now()
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	s.entries[key] = inMemoryValue[V]{
		value:  value,
		expiry: now.Add(s.maxAge),
	}
}

func (s *inMemoryStore[K, V]) GetOrSet(key K, value V) (currentValue V, set bool) {
	if v, ok := s.Get(key); ok {
		return v, false
	}

	var now time.Time
	if s.maxAge > 0 {
		now = time.Now()
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	e, ok := s.entries[key]
	if ok && !e.isExpired() {
		return e.value, false
	}

	s.entries[key] = inMemoryValue[V]{
		value:  value,
		expiry: now.Add(s.maxAge),
	}

	return value, true
}

type inMemoryValue[V any] struct {
	value  V
	expiry time.Time
}

func (v inMemoryValue[V]) isExpired() bool {
	return !v.expiry.IsZero() && v.expiry.Before(time.Now())
}
