package metrics

import "sync"

// Counter stores integer metrics identified by name.
type Counter struct {
	mu     sync.Mutex
	values map[string]int64
}

// NewCounter creates a new Counter.
func NewCounter() *Counter {
	return &Counter{values: make(map[string]int64)}
}

// Inc increments the named counter.
func (c *Counter) Inc(name string) {
	c.mu.Lock()
	c.values[name]++
	c.mu.Unlock()
}

// Get returns the value of the named counter.
func (c *Counter) Get(name string) int64 {
	c.mu.Lock()
	v := c.values[name]
	c.mu.Unlock()
	return v
}

// All returns a copy of all counters.
func (c *Counter) All() map[string]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	m := make(map[string]int64, len(c.values))
	for k, v := range c.values {
		m[k] = v
	}
	return m
}
