// Package eventual provides values that eventually have a value.
package eventual

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	intFalse = 0
	intTrue  = 1
)

// Value is an eventual value, meaning that callers wishing to access the value
// block until the value is available.
type Value interface {
	// Set sets this Value to the given val.
	Set(val interface{})

	// Get waits for the value to be set and returns it, or returns nil if it
	// times out or Close() is called. valid will be false in latter case.
	Get(timeout time.Duration) (ret interface{}, valid bool)
}

// Getter is a functional interface for the Value.Get function
type Getter func(time.Duration) (interface{}, bool)

type value struct {
	val      atomic.Value
	firstSet int32
	waiters  []chan interface{}
	mutex    sync.Mutex
}

// NewValue creates a new Value.
func NewValue() Value {
	return &value{waiters: make([]chan interface{}, 0, 10)}
}

// DefaultGetter builds a Getter that always returns the supplied value.
func DefaultGetter(val interface{}) Getter {
	return func(time.Duration) (interface{}, bool) {
		return val, true
	}
}

func (v *value) Set(val interface{}) {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	v.val.Store(val)
	atomic.StoreInt32(&v.firstSet, intTrue)
	// Notify anyone waiting for value
	for _, waiter := range v.waiters {
		waiter <- val
	}
}

func (v *value) Get(timeout time.Duration) (ret interface{}, valid bool) {
	// First check for existing value using atomic operations (for speed)
	if atomic.LoadInt32(&v.firstSet) == intTrue {
		return v.val.Load(), true
	}

	// If we didn't find an existing value, try again but this time using locking
	var valCh chan interface{}
	v.mutex.Lock()
	if atomic.LoadInt32(&v.firstSet) == intTrue {
		// Value found, use it
		r := v.val.Load()
		v.mutex.Unlock()
		return r, true
	} else {
		// Value not found, register to be notified once value is set
		valCh = make(chan interface{}, 1)
		v.waiters = append(v.waiters, valCh)
		v.mutex.Unlock()
	}

	// Wait up to timeout for value to get set
	select {
	case v := <-valCh:
		return v, true
	case <-time.After(timeout):
		return nil, false
	}
}
