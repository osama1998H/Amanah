package metrics

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Counter stores integer metrics identified by name.
type Counter struct {
	mu     sync.RWMutex
	values map[string]*int64
	labels map[string]map[string]string // name -> labels
}

// NewCounter creates a new Counter.
func NewCounter() *Counter {
	return &Counter{
		values: make(map[string]*int64),
		labels: make(map[string]map[string]string),
	}
}

// Inc increments the named counter.
func (c *Counter) Inc(name string) {
	c.Add(name, 1)
}

// Add adds a value to the named counter.
func (c *Counter) Add(name string, delta int64) {
	c.mu.RLock()
	ptr, exists := c.values[name]
	c.mu.RUnlock()

	if exists {
		atomic.AddInt64(ptr, delta)
		return
	}

	c.mu.Lock()
	if ptr, exists = c.values[name]; !exists {
		v := delta
		c.values[name] = &v
	} else {
		atomic.AddInt64(ptr, delta)
	}
	c.mu.Unlock()
}

// IncWithLabels increments a counter with labels
func (c *Counter) IncWithLabels(name string, labels map[string]string) {
	key := c.formatKey(name, labels)
	c.Inc(key)

	c.mu.Lock()
	c.labels[key] = labels
	c.mu.Unlock()
}

// formatKey creates a unique key from name and labels
func (c *Counter) formatKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(labels))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}

	return name + "{" + strings.Join(parts, ",") + "}"
}

// Get returns the value of the named counter.
func (c *Counter) Get(name string) int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if ptr, exists := c.values[name]; exists {
		return atomic.LoadInt64(ptr)
	}
	return 0
}

// Reset resets a counter to zero
func (c *Counter) Reset(name string) {
	c.mu.Lock()
	if ptr, exists := c.values[name]; exists {
		atomic.StoreInt64(ptr, 0)
	}
	c.mu.Unlock()
}

// All returns a copy of all counters.
func (c *Counter) All() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	m := make(map[string]int64, len(c.values))
	for k, ptr := range c.values {
		m[k] = atomic.LoadInt64(ptr)
	}
	return m
}

// Gauge stores float64 metrics that can go up and down
type Gauge struct {
	mu     sync.RWMutex
	values map[string]float64
}

// NewGauge creates a new Gauge
func NewGauge() *Gauge {
	return &Gauge{
		values: make(map[string]float64),
	}
}

// Set sets the gauge value
func (g *Gauge) Set(name string, value float64) {
	g.mu.Lock()
	g.values[name] = value
	g.mu.Unlock()
}

// Inc increments the gauge
func (g *Gauge) Inc(name string) {
	g.Add(name, 1)
}

// Dec decrements the gauge
func (g *Gauge) Dec(name string) {
	g.Add(name, -1)
}

// Add adds a value to the gauge
func (g *Gauge) Add(name string, delta float64) {
	g.mu.Lock()
	g.values[name] += delta
	g.mu.Unlock()
}

// Get returns the gauge value
func (g *Gauge) Get(name string) float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.values[name]
}

// All returns all gauge values
func (g *Gauge) All() map[string]float64 {
	g.mu.RLock()
	defer g.mu.RUnlock()

	m := make(map[string]float64, len(g.values))
	for k, v := range g.values {
		m[k] = v
	}
	return m
}

// Histogram collects observations and provides percentile statistics
type Histogram struct {
	mu           sync.Mutex
	name         string
	observations []float64
	sum          float64
	count        int64
	buckets      []float64 // Bucket boundaries
	bucketCounts []int64   // Counts per bucket
}

// DefaultBuckets are the default histogram buckets (in milliseconds for latency)
var DefaultBuckets = []float64{5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000}

// NewHistogram creates a new Histogram
func NewHistogram(name string, buckets []float64) *Histogram {
	if buckets == nil {
		buckets = DefaultBuckets
	}

	return &Histogram{
		name:         name,
		observations: make([]float64, 0, 1000),
		buckets:      buckets,
		bucketCounts: make([]int64, len(buckets)+1), // +1 for +Inf
	}
}

// Observe records an observation
func (h *Histogram) Observe(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.observations = append(h.observations, value)
	h.sum += value
	h.count++

	// Update bucket counts
	for i, boundary := range h.buckets {
		if value <= boundary {
			h.bucketCounts[i]++
			break
		}
		if i == len(h.buckets)-1 {
			h.bucketCounts[len(h.buckets)]++ // +Inf bucket
		}
	}
}

// ObserveDuration records a duration observation
func (h *Histogram) ObserveDuration(d time.Duration) {
	h.Observe(float64(d.Milliseconds()))
}

// Sum returns the sum of all observations
func (h *Histogram) Sum() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sum
}

// Count returns the number of observations
func (h *Histogram) Count() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// Mean returns the mean of all observations
func (h *Histogram) Mean() float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.count == 0 {
		return 0
	}
	return h.sum / float64(h.count)
}

// Percentile returns the percentile value (0-100)
func (h *Histogram) Percentile(p float64) float64 {
	h.mu.Lock()
	defer h.mu.Unlock()

	if len(h.observations) == 0 {
		return 0
	}

	sorted := make([]float64, len(h.observations))
	copy(sorted, h.observations)
	sort.Float64s(sorted)

	index := int(float64(len(sorted)-1) * p / 100)
	return sorted[index]
}

// Reset resets the histogram
func (h *Histogram) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.observations = h.observations[:0]
	h.sum = 0
	h.count = 0
	for i := range h.bucketCounts {
		h.bucketCounts[i] = 0
	}
}

// Registry holds all metrics
type Registry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	histograms map[string]*Histogram

	// Predefined metrics
	Requests       *Counter
	RequestLatency *Histogram
	Errors         *Counter
	ActiveConns    *Gauge
}

// NewRegistry creates a new metrics registry
func NewRegistry() *Registry {
	r := &Registry{
		counters:   make(map[string]*Counter),
		gauges:     make(map[string]*Gauge),
		histograms: make(map[string]*Histogram),
	}

	// Initialize predefined metrics
	r.Requests = r.Counter("http_requests_total")
	r.RequestLatency = r.Histogram("http_request_duration_ms", nil)
	r.Errors = r.Counter("errors_total")
	r.ActiveConns = r.Gauge("active_connections")

	return r
}

// Counter gets or creates a counter
func (r *Registry) Counter(name string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if c, exists := r.counters[name]; exists {
		return c
	}

	c := NewCounter()
	r.counters[name] = c
	return c
}

// Gauge gets or creates a gauge
func (r *Registry) Gauge(name string) *Gauge {
	r.mu.Lock()
	defer r.mu.Unlock()

	if g, exists := r.gauges[name]; exists {
		return g
	}

	g := NewGauge()
	r.gauges[name] = g
	return g
}

// Histogram gets or creates a histogram
func (r *Registry) Histogram(name string, buckets []float64) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()

	if h, exists := r.histograms[name]; exists {
		return h
	}

	h := NewHistogram(name, buckets)
	r.histograms[name] = h
	return h
}

// Snapshot represents a point-in-time snapshot of all metrics
type Snapshot struct {
	Timestamp  time.Time                     `json:"timestamp"`
	Counters   map[string]map[string]int64   `json:"counters"`
	Gauges     map[string]map[string]float64 `json:"gauges"`
	Histograms map[string]HistogramStats     `json:"histograms"`
	System     SystemMetrics                 `json:"system"`
}

// HistogramStats holds histogram statistics
type HistogramStats struct {
	Count int64   `json:"count"`
	Sum   float64 `json:"sum"`
	Mean  float64 `json:"mean"`
	P50   float64 `json:"p50"`
	P90   float64 `json:"p90"`
	P95   float64 `json:"p95"`
	P99   float64 `json:"p99"`
}

// SystemMetrics holds system-level metrics
type SystemMetrics struct {
	Goroutines    int     `json:"goroutines"`
	HeapAlloc     uint64  `json:"heap_alloc_bytes"`
	HeapSys       uint64  `json:"heap_sys_bytes"`
	HeapObjects   uint64  `json:"heap_objects"`
	GCPauseNs     uint64  `json:"gc_pause_ns"`
	NumGC         uint32  `json:"num_gc"`
	CPUCores      int     `json:"cpu_cores"`
}

// Snapshot returns a point-in-time snapshot of all metrics
func (r *Registry) Snapshot() *Snapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snap := &Snapshot{
		Timestamp:  time.Now(),
		Counters:   make(map[string]map[string]int64),
		Gauges:     make(map[string]map[string]float64),
		Histograms: make(map[string]HistogramStats),
	}

	// Collect counters
	for name, counter := range r.counters {
		snap.Counters[name] = counter.All()
	}

	// Collect gauges
	for name, gauge := range r.gauges {
		snap.Gauges[name] = gauge.All()
	}

	// Collect histograms
	for name, histogram := range r.histograms {
		snap.Histograms[name] = HistogramStats{
			Count: histogram.Count(),
			Sum:   histogram.Sum(),
			Mean:  histogram.Mean(),
			P50:   histogram.Percentile(50),
			P90:   histogram.Percentile(90),
			P95:   histogram.Percentile(95),
			P99:   histogram.Percentile(99),
		}
	}

	// Collect system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	snap.System = SystemMetrics{
		Goroutines:  runtime.NumGoroutine(),
		HeapAlloc:   m.HeapAlloc,
		HeapSys:     m.HeapSys,
		HeapObjects: m.HeapObjects,
		GCPauseNs:   m.PauseNs[(m.NumGC+255)%256],
		NumGC:       m.NumGC,
		CPUCores:    runtime.NumCPU(),
	}

	return snap
}

// Handler returns an HTTP handler for the metrics endpoint
func (r *Registry) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		format := req.URL.Query().Get("format")

		if format == "prometheus" {
			r.writePrometheus(w)
			return
		}

		// Default to JSON
		snap := r.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(snap)
	})
}

// writePrometheus writes metrics in Prometheus format
func (r *Registry) writePrometheus(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Write counters
	for name, counter := range r.counters {
		for metricName, value := range counter.All() {
			if strings.Contains(metricName, "{") {
				fmt.Fprintf(w, "%s %d\n", metricName, value)
			} else {
				fmt.Fprintf(w, "%s_%s %d\n", name, metricName, value)
			}
		}
	}

	// Write gauges
	for name, gauge := range r.gauges {
		for metricName, value := range gauge.All() {
			fmt.Fprintf(w, "%s_%s %f\n", name, metricName, value)
		}
	}

	// Write histograms
	for name, histogram := range r.histograms {
		fmt.Fprintf(w, "%s_count %d\n", name, histogram.Count())
		fmt.Fprintf(w, "%s_sum %f\n", name, histogram.Sum())
		fmt.Fprintf(w, "%s{quantile=\"0.5\"} %f\n", name, histogram.Percentile(50))
		fmt.Fprintf(w, "%s{quantile=\"0.9\"} %f\n", name, histogram.Percentile(90))
		fmt.Fprintf(w, "%s{quantile=\"0.95\"} %f\n", name, histogram.Percentile(95))
		fmt.Fprintf(w, "%s{quantile=\"0.99\"} %f\n", name, histogram.Percentile(99))
	}

	// Write system metrics
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fmt.Fprintf(w, "go_goroutines %d\n", runtime.NumGoroutine())
	fmt.Fprintf(w, "go_memstats_heap_alloc_bytes %d\n", m.HeapAlloc)
	fmt.Fprintf(w, "go_memstats_heap_sys_bytes %d\n", m.HeapSys)
	fmt.Fprintf(w, "go_memstats_heap_objects %d\n", m.HeapObjects)
	fmt.Fprintf(w, "go_gc_duration_seconds %f\n", float64(m.PauseNs[(m.NumGC+255)%256])/1e9)
}

// Global registry
var defaultRegistry = NewRegistry()

// GetRegistry returns the default registry
func GetRegistry() *Registry {
	return defaultRegistry
}

// RecordRequest records an HTTP request metric
func RecordRequest(method, path string, statusCode int, duration time.Duration) {
	labels := map[string]string{
		"method": method,
		"path":   path,
		"status": fmt.Sprintf("%d", statusCode),
	}

	defaultRegistry.Requests.IncWithLabels("http_requests", labels)
	defaultRegistry.RequestLatency.ObserveDuration(duration)

	if statusCode >= 400 {
		defaultRegistry.Errors.IncWithLabels("http_errors", labels)
	}
}

// Timer helps measure duration
type Timer struct {
	start time.Time
	hist  *Histogram
}

// NewTimer creates a new timer
func NewTimer(hist *Histogram) *Timer {
	return &Timer{
		start: time.Now(),
		hist:  hist,
	}
}

// ObserveDuration records the duration since the timer started
func (t *Timer) ObserveDuration() time.Duration {
	d := time.Since(t.start)
	if t.hist != nil {
		t.hist.ObserveDuration(d)
	}
	return d
}
