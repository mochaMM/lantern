// Package eventual provides values that eventually have a value.
package eventual

import (
	"sync"
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
	val      interface{}
	firstSet bool
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
	v.val = val
	v.firstSet = true
	// Notify anyone waiting for value
	for _, waiter := range v.waiters {
		waiter <- val
	}
}

func (v *value) Get(timeout time.Duration) (ret interface{}, valid bool) {
	var valCh chan interface{}
	v.mutex.Lock()
	if v.firstSet {
		r := v.val
		v.mutex.Unlock()
		return r, true
	} else {
		valCh = make(chan interface{}, 1)
		v.waiters = append(v.waiters, valCh)
		v.mutex.Unlock()
	}

	select {
	case v := <-valCh:
		return v, true
	case <-time.After(timeout):
		return nil, false
	}
}
