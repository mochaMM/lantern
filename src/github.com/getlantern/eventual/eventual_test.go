package eventual

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getlantern/grtrack"
	"github.com/stretchr/testify/assert"
)

const (
	concurrency = 200
)

func TestSingle(t *testing.T) {
	checkGoroutines := grtrack.Start()
	v := NewValue()
	go func() {
		time.Sleep(20 * time.Millisecond)
		v.Set("hi")
	}()

	r, ok := v.Get(10 * time.Millisecond)
	assert.False(t, ok, "Get with short timeout should have timed out")

	r, ok = v.Get(20 * time.Millisecond)
	assert.True(t, ok, "Get with longer timeout should have succeed")
	assert.Equal(t, "hi", r, "Wrong result")

	time.Sleep(50 * time.Millisecond)
	checkGoroutines(t)
}

func TestNoSet(t *testing.T) {
	checkGoroutines := grtrack.Start()
	v := NewValue()

	_, ok := v.Get(10 * time.Millisecond)
	assert.False(t, ok, "Get before setting value should not be okay")

	time.Sleep(50 * time.Millisecond)
	checkGoroutines(t)
}

func TestCancelImmediate(t *testing.T) {
	v := NewValue()
	go func() {
		time.Sleep(10 * time.Millisecond)
		v.Cancel()
	}()

	_, ok := v.Get(200 * time.Millisecond)
	assert.False(t, ok, "Get after cancel should have failed")
}

func TestCancelAfterSet(t *testing.T) {
	v := NewValue()
	v.Set(5)
	r, ok := v.Get(10 * time.Millisecond)
	assert.True(t, ok, "Get should have succeeded")
	assert.Equal(t, 5, r, "Get got wrong value")

	v.Cancel()
	_, ok = v.Get(10 * time.Millisecond)
	assert.False(t, ok, "Get after cancel should have failed")
}

func BenchmarkGet(b *testing.B) {
	v := NewValue()
	go func() {
		time.Sleep(20 * time.Millisecond)
		v.Set("hi")
	}()

	for i := 0; i < b.N; i++ {
		v.Get(20 * time.Millisecond)
	}
}

func TestConcurrent(t *testing.T) {
	checkGoroutines := grtrack.Start()
	v := NewValue()

	var sets int32

	go func() {
		var wg sync.WaitGroup
		wg.Add(1)
		// Do some concurrent setting to make sure that it works
		for i := 0; i < concurrency; i++ {
			go func() {
				// Wait for waitGroup so that all goroutines run at basically the same
				// time.
				wg.Wait()
				v.Set("hi")
				atomic.AddInt32(&sets, 1)
			}()
		}
		wg.Done()
	}()

	for i := 0; i < concurrency; i++ {
		go func() {
			r, ok := v.Get(200 * time.Millisecond)
			assert.True(t, ok, "Get should have succeed")
			assert.Equal(t, "hi", r, "Wrong result")
		}()
	}

	time.Sleep(50 * time.Millisecond)
	assert.EqualValues(t, concurrency, atomic.LoadInt32(&sets), "Wrong number of successful Sets")
	checkGoroutines(t)
}
