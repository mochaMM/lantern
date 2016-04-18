// Package eventual provides values that eventually have a value.
package eventual

import (
	"sync"
	"sync/atomic"
	"time"
)

const (
	unset    = 0
	set      = 1
	canceled = 2
)

// Value is an eventual value, meaning that callers wishing to access the value
// block until the value is available.
type Value interface {
	// Set sets this Value to the given val.
	Set(val interface{})

	// Get waits for the value to be set and returns it, or returns nil if it
	// times out or Cancel() is called. valid will be false in latter case.
	Get(timeout time.Duration) (ret interface{}, valid bool)

	// Cancel cancels this value, signaling any waiting calls to Get() that no
	// value is coming. After canceling, all future calls to Get() will return
	// false.
	Cancel()
}

// Getter is a functional interface for the Value.Get function
type Getter func(time.Duration) (interface{}, bool)

type value struct {
	val     atomic.Value
	status  int32
	waiters []chan interface{}
	mutex   sync.Mutex
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
	atomic.StoreInt32(&v.status, set)

	if v.waiters != nil {
		// Notify anyone waiting for value
		for _, waiter := range v.waiters {
			waiter <- val
		}
		// Clear waiters
		v.waiters = nil
	}
}

func (v *value) Cancel() {
	v.mutex.Lock()
	defer v.mutex.Unlock()
	atomic.StoreInt32(&v.status, canceled)

	if v.waiters != nil {
		// Notify anyone waiting for value
		for _, waiter := range v.waiters {
			close(waiter)
		}
		// Clear waiters
		v.waiters = nil
	}
}

func (v *value) Get(timeout time.Duration) (ret interface{}, valid bool) {
	status := atomic.LoadInt32(&v.status)
	// First check for existing value using atomic operations (for speed)
	if status == canceled {
		// Value was canceled, return false
		return nil, false
	} else if status == set {
		// Value found, use it
		return v.val.Load(), true
	}

	// If we didn't find an existing value, try again but this time using locking
	v.mutex.Lock()
	status = atomic.LoadInt32(&v.status)
	if status == canceled {
		// Value was canceled, return false
		v.mutex.Unlock()
		return nil, false
	} else if status == set {
		// Value found, use it
		r := v.val.Load()
		v.mutex.Unlock()
		return r, true
	}

	// Value not found, register to be notified once value is set
	valCh := make(chan interface{}, 1)
	v.waiters = append(v.waiters, valCh)
	v.mutex.Unlock()

	// Wait up to timeout for value to get set
	select {
	case v, ok := <-valCh:
		return v, ok
	case <-time.After(timeout):
		return nil, false
	}
}
