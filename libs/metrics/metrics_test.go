package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestCounter(t *testing.T) {
	c := NewCounter()

	c.Inc("requests")
	c.Inc("requests")
	c.Inc("requests")

	if got := c.Get("requests"); got != 3 {
		t.Errorf("Expected 3, got %d", got)
	}

	c.Add("requests", 7)
	if got := c.Get("requests"); got != 10 {
		t.Errorf("Expected 10, got %d", got)
	}
}

func TestCounterConcurrency(t *testing.T) {
	c := NewCounter()
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.Inc("concurrent")
		}()
	}

	wg.Wait()

	if got := c.Get("concurrent"); got != 1000 {
		t.Errorf("Expected 1000, got %d", got)
	}
}

func TestCounterWithLabels(t *testing.T) {
	c := NewCounter()

	labels := map[string]string{
		"method": "GET",
		"path":   "/api/users",
	}

	c.IncWithLabels("requests", labels)
	c.IncWithLabels("requests", labels)

	key := c.formatKey("requests", labels)
	if got := c.Get(key); got != 2 {
		t.Errorf("Expected 2, got %d", got)
	}
}

func TestCounterReset(t *testing.T) {
	c := NewCounter()

	c.Inc("test")
	c.Inc("test")
	c.Reset("test")

	if got := c.Get("test"); got != 0 {
		t.Errorf("Expected 0 after reset, got %d", got)
	}
}

func TestCounterAll(t *testing.T) {
	c := NewCounter()

	c.Inc("a")
	c.Inc("b")
	c.Inc("c")

	all := c.All()
	if len(all) != 3 {
		t.Errorf("Expected 3 counters, got %d", len(all))
	}
}

func TestGauge(t *testing.T) {
	g := NewGauge()

	g.Set("temperature", 25.5)
	if got := g.Get("temperature"); got != 25.5 {
		t.Errorf("Expected 25.5, got %f", got)
	}

	g.Inc("temperature")
	if got := g.Get("temperature"); got != 26.5 {
		t.Errorf("Expected 26.5, got %f", got)
	}

	g.Dec("temperature")
	if got := g.Get("temperature"); got != 25.5 {
		t.Errorf("Expected 25.5, got %f", got)
	}

	g.Add("temperature", -10)
	if got := g.Get("temperature"); got != 15.5 {
		t.Errorf("Expected 15.5, got %f", got)
	}
}

func TestGaugeAll(t *testing.T) {
	g := NewGauge()

	g.Set("a", 1.0)
	g.Set("b", 2.0)

	all := g.All()
	if len(all) != 2 {
		t.Errorf("Expected 2 gauges, got %d", len(all))
	}
}

func TestHistogram(t *testing.T) {
	h := NewHistogram("latency", nil)

	// Add some observations
	values := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}
	for _, v := range values {
		h.Observe(v)
	}

	if h.Count() != 10 {
		t.Errorf("Expected count 10, got %d", h.Count())
	}

	if h.Sum() != 550 {
		t.Errorf("Expected sum 550, got %f", h.Sum())
	}

	if h.Mean() != 55 {
		t.Errorf("Expected mean 55, got %f", h.Mean())
	}

	// Check percentiles
	p50 := h.Percentile(50)
	if p50 < 40 || p50 > 60 {
		t.Errorf("Expected p50 around 50, got %f", p50)
	}

	p90 := h.Percentile(90)
	if p90 < 80 || p90 > 100 {
		t.Errorf("Expected p90 around 90, got %f", p90)
	}
}

func TestHistogramEmpty(t *testing.T) {
	h := NewHistogram("empty", nil)

	if h.Mean() != 0 {
		t.Error("Empty histogram should have 0 mean")
	}

	if h.Percentile(50) != 0 {
		t.Error("Empty histogram should have 0 percentile")
	}
}

func TestHistogramReset(t *testing.T) {
	h := NewHistogram("test", nil)

	h.Observe(10)
	h.Observe(20)
	h.Reset()

	if h.Count() != 0 {
		t.Errorf("Expected count 0 after reset, got %d", h.Count())
	}

	if h.Sum() != 0 {
		t.Errorf("Expected sum 0 after reset, got %f", h.Sum())
	}
}

func TestHistogramObserveDuration(t *testing.T) {
	h := NewHistogram("duration", nil)

	h.ObserveDuration(100 * time.Millisecond)
	h.ObserveDuration(200 * time.Millisecond)

	if h.Count() != 2 {
		t.Errorf("Expected count 2, got %d", h.Count())
	}

	if h.Sum() != 300 {
		t.Errorf("Expected sum 300, got %f", h.Sum())
	}
}

func TestRegistry(t *testing.T) {
	r := NewRegistry()

	// Get counters
	c1 := r.Counter("requests")
	c2 := r.Counter("requests")
	if c1 != c2 {
		t.Error("Same counter name should return same counter")
	}

	// Get gauges
	g1 := r.Gauge("connections")
	g2 := r.Gauge("connections")
	if g1 != g2 {
		t.Error("Same gauge name should return same gauge")
	}

	// Get histograms
	h1 := r.Histogram("latency", nil)
	h2 := r.Histogram("latency", nil)
	if h1 != h2 {
		t.Error("Same histogram name should return same histogram")
	}
}

func TestRegistryPredefinedMetrics(t *testing.T) {
	r := NewRegistry()

	if r.Requests == nil {
		t.Error("Requests counter should be initialized")
	}
	if r.RequestLatency == nil {
		t.Error("RequestLatency histogram should be initialized")
	}
	if r.Errors == nil {
		t.Error("Errors counter should be initialized")
	}
	if r.ActiveConns == nil {
		t.Error("ActiveConns gauge should be initialized")
	}
}

func TestRegistrySnapshot(t *testing.T) {
	r := NewRegistry()

	r.Counter("test").Inc("value")
	r.Gauge("test").Set("value", 42)
	r.Histogram("test", nil).Observe(100)

	snap := r.Snapshot()

	if snap.Timestamp.IsZero() {
		t.Error("Snapshot timestamp should be set")
	}

	if len(snap.Counters) == 0 {
		t.Error("Snapshot should have counters")
	}

	if len(snap.Gauges) == 0 {
		t.Error("Snapshot should have gauges")
	}

	if len(snap.Histograms) == 0 {
		t.Error("Snapshot should have histograms")
	}

	// Check system metrics
	if snap.System.Goroutines <= 0 {
		t.Error("Goroutines should be positive")
	}

	if snap.System.CPUCores <= 0 {
		t.Error("CPUCores should be positive")
	}
}

func TestRegistryJSONHandler(t *testing.T) {
	r := NewRegistry()
	r.Counter("test").Inc("requests")

	handler := r.Handler()

	req := httptest.NewRequest("GET", "/metrics", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Header().Get("Content-Type"), "application/json") {
		t.Error("Expected JSON content type")
	}
}

func TestRegistryPrometheusHandler(t *testing.T) {
	r := NewRegistry()
	r.Counter("test").Inc("requests")
	r.Gauge("test").Set("connections", 10)
	r.Histogram("test", nil).Observe(100)

	handler := r.Handler()

	req := httptest.NewRequest("GET", "/metrics?format=prometheus", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Header().Get("Content-Type"), "text/plain") {
		t.Error("Expected text/plain content type")
	}

	body := rr.Body.String()
	if !strings.Contains(body, "go_goroutines") {
		t.Error("Expected go_goroutines metric")
	}
}

func TestRecordRequest(t *testing.T) {
	// Reset default registry
	defaultRegistry = NewRegistry()

	RecordRequest("GET", "/api/users", 200, 100*time.Millisecond)
	RecordRequest("POST", "/api/users", 201, 150*time.Millisecond)
	RecordRequest("GET", "/api/error", 500, 50*time.Millisecond)

	if defaultRegistry.RequestLatency.Count() != 3 {
		t.Errorf("Expected 3 latency observations, got %d", defaultRegistry.RequestLatency.Count())
	}
}

func TestTimer(t *testing.T) {
	h := NewHistogram("operation", nil)

	timer := NewTimer(h)
	time.Sleep(10 * time.Millisecond)
	duration := timer.ObserveDuration()

	if duration < 10*time.Millisecond {
		t.Errorf("Expected at least 10ms, got %v", duration)
	}

	if h.Count() != 1 {
		t.Errorf("Expected 1 observation, got %d", h.Count())
	}
}

func TestTimerWithoutHistogram(t *testing.T) {
	timer := NewTimer(nil)
	time.Sleep(5 * time.Millisecond)
	duration := timer.ObserveDuration()

	if duration < 5*time.Millisecond {
		t.Errorf("Expected at least 5ms, got %v", duration)
	}
}

func TestGetRegistry(t *testing.T) {
	r := GetRegistry()
	if r == nil {
		t.Error("GetRegistry should not return nil")
	}
}

func TestHistogramBuckets(t *testing.T) {
	buckets := []float64{10, 50, 100, 500, 1000}
	h := NewHistogram("custom_buckets", buckets)

	// Observations in different buckets
	h.Observe(5)   // <= 10 bucket
	h.Observe(25)  // <= 50 bucket
	h.Observe(75)  // <= 100 bucket
	h.Observe(250) // <= 500 bucket
	h.Observe(750) // <= 1000 bucket
	h.Observe(2000) // +Inf bucket

	if h.Count() != 6 {
		t.Errorf("Expected 6 observations, got %d", h.Count())
	}
}

func TestCounterFormatKey(t *testing.T) {
	c := NewCounter()

	// Test with no labels
	key := c.formatKey("metric", nil)
	if key != "metric" {
		t.Errorf("Expected 'metric', got %s", key)
	}

	// Test with labels (should be sorted)
	labels := map[string]string{
		"method": "GET",
		"path":   "/api",
	}
	key = c.formatKey("requests", labels)

	// Labels should be sorted alphabetically
	if !strings.Contains(key, "method=GET") || !strings.Contains(key, "path=/api") {
		t.Errorf("Unexpected key format: %s", key)
	}
}

func TestSystemMetrics(t *testing.T) {
	r := NewRegistry()
	snap := r.Snapshot()

	if snap.System.Goroutines < 1 {
		t.Error("Should have at least 1 goroutine")
	}

	if snap.System.HeapAlloc == 0 {
		t.Error("HeapAlloc should not be 0")
	}

	if snap.System.CPUCores < 1 {
		t.Error("Should have at least 1 CPU core")
	}
}
