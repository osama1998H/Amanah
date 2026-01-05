package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name        string        `json:"name"`
	URL         string        `json:"url"`
	Status      HealthStatus  `json:"status"`
	Latency     time.Duration `json:"latency_ms"`
	LastChecked time.Time     `json:"last_checked"`
	LastError   string        `json:"last_error,omitempty"`
	Consecutive int           `json:"consecutive_failures,omitempty"`
}

// HealthStatus represents the health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnknown   HealthStatus = "unknown"
)

// AggregatedHealth represents the aggregated health of all services
type AggregatedHealth struct {
	Status     HealthStatus     `json:"status"`
	Services   []*ServiceHealth `json:"services"`
	Healthy    int              `json:"healthy"`
	Unhealthy  int              `json:"unhealthy"`
	Degraded   int              `json:"degraded"`
	Unknown    int              `json:"unknown"`
	CheckedAt  time.Time        `json:"checked_at"`
}

// HealthChecker checks the health of backend services
type HealthChecker struct {
	mu           sync.RWMutex
	services     map[string]*serviceHealthCheck
	client       *http.Client
	checkPeriod  time.Duration
	stop         chan struct{}
	running      bool
}

// serviceHealthCheck holds health check state for a service
type serviceHealthCheck struct {
	name         string
	url          string
	status       HealthStatus
	latency      time.Duration
	lastChecked  time.Time
	lastError    string
	consecutive  int // Consecutive failures
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(checkPeriod time.Duration) *HealthChecker {
	if checkPeriod <= 0 {
		checkPeriod = 10 * time.Second
	}

	return &HealthChecker{
		services:    make(map[string]*serviceHealthCheck),
		checkPeriod: checkPeriod,
		stop:        make(chan struct{}),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// AddService adds a service to monitor
func (hc *HealthChecker) AddService(name, healthURL string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.services[name] = &serviceHealthCheck{
		name:   name,
		url:    healthURL,
		status: HealthStatusUnknown,
	}
}

// RemoveService removes a service from monitoring
func (hc *HealthChecker) RemoveService(name string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	delete(hc.services, name)
}

// Start starts the health checker
func (hc *HealthChecker) Start() {
	hc.mu.Lock()
	if hc.running {
		hc.mu.Unlock()
		return
	}
	hc.running = true
	hc.stop = make(chan struct{})
	hc.mu.Unlock()

	// Do initial check
	hc.CheckAll()

	// Start periodic checks
	go hc.run()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	if !hc.running {
		return
	}

	close(hc.stop)
	hc.running = false
}

// run performs periodic health checks
func (hc *HealthChecker) run() {
	ticker := time.NewTicker(hc.checkPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.CheckAll()
		case <-hc.stop:
			return
		}
	}
}

// CheckAll checks all registered services
func (hc *HealthChecker) CheckAll() {
	hc.mu.RLock()
	services := make([]*serviceHealthCheck, 0, len(hc.services))
	for _, svc := range hc.services {
		services = append(services, svc)
	}
	hc.mu.RUnlock()

	var wg sync.WaitGroup
	for _, svc := range services {
		wg.Add(1)
		go func(s *serviceHealthCheck) {
			defer wg.Done()
			hc.checkService(s)
		}(svc)
	}
	wg.Wait()
}

// checkService checks a single service
func (hc *HealthChecker) checkService(svc *serviceHealthCheck) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, svc.url, nil)
	if err != nil {
		hc.updateServiceStatus(svc, HealthStatusUnhealthy, 0, err.Error())
		return
	}

	resp, err := hc.client.Do(req)
	latency := time.Since(start)

	if err != nil {
		hc.updateServiceStatus(svc, HealthStatusUnhealthy, latency, err.Error())
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		hc.updateServiceStatus(svc, HealthStatusHealthy, latency, "")
	} else if resp.StatusCode >= 500 {
		hc.updateServiceStatus(svc, HealthStatusUnhealthy, latency, "HTTP "+resp.Status)
	} else {
		hc.updateServiceStatus(svc, HealthStatusDegraded, latency, "HTTP "+resp.Status)
	}
}

// updateServiceStatus updates the health status of a service
func (hc *HealthChecker) updateServiceStatus(svc *serviceHealthCheck, status HealthStatus, latency time.Duration, errMsg string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	svc.status = status
	svc.latency = latency
	svc.lastChecked = time.Now()
	svc.lastError = errMsg

	if status == HealthStatusHealthy {
		svc.consecutive = 0
	} else {
		svc.consecutive++
	}
}

// GetServiceHealth returns the health of a specific service
func (hc *HealthChecker) GetServiceHealth(name string) (*ServiceHealth, bool) {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	svc, exists := hc.services[name]
	if !exists {
		return nil, false
	}

	return &ServiceHealth{
		Name:        svc.name,
		URL:         svc.url,
		Status:      svc.status,
		Latency:     svc.latency / time.Millisecond,
		LastChecked: svc.lastChecked,
		LastError:   svc.lastError,
		Consecutive: svc.consecutive,
	}, true
}

// GetAggregatedHealth returns the aggregated health of all services
func (hc *HealthChecker) GetAggregatedHealth() *AggregatedHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	health := &AggregatedHealth{
		Status:    HealthStatusHealthy,
		Services:  make([]*ServiceHealth, 0, len(hc.services)),
		CheckedAt: time.Now(),
	}

	for _, svc := range hc.services {
		sh := &ServiceHealth{
			Name:        svc.name,
			URL:         svc.url,
			Status:      svc.status,
			Latency:     svc.latency / time.Millisecond,
			LastChecked: svc.lastChecked,
			LastError:   svc.lastError,
			Consecutive: svc.consecutive,
		}
		health.Services = append(health.Services, sh)

		switch svc.status {
		case HealthStatusHealthy:
			health.Healthy++
		case HealthStatusUnhealthy:
			health.Unhealthy++
		case HealthStatusDegraded:
			health.Degraded++
		default:
			health.Unknown++
		}
	}

	// Determine overall status
	if health.Unhealthy > 0 {
		health.Status = HealthStatusUnhealthy
	} else if health.Degraded > 0 || health.Unknown > 0 {
		health.Status = HealthStatusDegraded
	}

	return health
}

// HealthHandler returns an HTTP handler for the health endpoint
func (hc *HealthChecker) HealthHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		health := hc.GetAggregatedHealth()

		w.Header().Set("Content-Type", "application/json")

		if health.Status == HealthStatusHealthy {
			w.WriteHeader(http.StatusOK)
		} else if health.Status == HealthStatusDegraded {
			w.WriteHeader(http.StatusOK) // Still OK but degraded
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}

		json.NewEncoder(w).Encode(health)
	})
}

// LivenessHandler returns a simple liveness check handler
func LivenessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"status": "alive",
		})
	})
}

// ReadinessHandler returns a readiness check handler
func (hc *HealthChecker) ReadinessHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		health := hc.GetAggregatedHealth()

		w.Header().Set("Content-Type", "application/json")

		// Not ready if any service is unhealthy
		if health.Status == HealthStatusUnhealthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"status":  "not_ready",
				"details": health,
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "ready",
			"details": health,
		})
	})
}
