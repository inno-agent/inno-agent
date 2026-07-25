package telemetry

import (
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	initOnce sync.Once
	registry *prometheus.Registry
	env      string

	httpRequests     *prometheus.CounterVec
	httpDuration     *prometheus.HistogramVec
	httpInFlight     *prometheus.GaugeVec
	errorTotal       *prometheus.CounterVec
	serviceUp        *prometheus.GaugeVec
	healthcheckTotal *prometheus.CounterVec

	aliasHTTPRequests *prometheus.CounterVec
	aliasHTTPDuration *prometheus.HistogramVec
	aliasHTTPInFlight *prometheus.GaugeVec
	aliasErrors       *prometheus.CounterVec
)

// Init registers collectors for a service. Call once at startup.
func Init(serviceName string) {
	initOnce.Do(func() {
		env = os.Getenv("METRICS_ENV")
		if env == "" {
			env = "local"
		}

		registry = prometheus.NewRegistry()
		registry.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		)

		httpRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "service_http_requests_total",
			Help: "Total HTTP requests handled by the service.",
		}, []string{"service", "method", "path", "status", "env"})
		httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "service_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"service", "method", "path", "status", "env"})
		httpInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "service_http_requests_in_flight",
			Help: "In-flight HTTP requests.",
		}, []string{"service", "env"})
		errorTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "service_errors_total",
			Help: "Total application errors by type.",
		}, []string{"service", "type", "env"})
		serviceUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "service_up",
			Help: "Service availability (1 = up).",
		}, []string{"service", "env"})
		healthcheckTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "service_healthcheck_total",
			Help: "Healthcheck probe results.",
		}, []string{"service", "status", "env"})

		registry.MustRegister(httpRequests, httpDuration, httpInFlight, errorTotal, serviceUp, healthcheckTotal)

		if prefix := aliasPrefix(serviceName); prefix != "" {
			aliasHTTPRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prefix + "_http_requests_total",
				Help: "HTTP requests for " + serviceName + ".",
			}, []string{"method", "path", "status", "env"})
			aliasHTTPDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
				Name:    prefix + "_http_request_duration_seconds",
				Help:    "HTTP latency for " + serviceName + ".",
				Buckets: prometheus.DefBuckets,
			}, []string{"method", "path", "status", "env"})
			aliasHTTPInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
				Name: prefix + "_http_requests_in_flight",
				Help: "In-flight HTTP requests for " + serviceName + ".",
			}, []string{"env"})
			registry.MustRegister(aliasHTTPRequests, aliasHTTPDuration, aliasHTTPInFlight)
			aliasErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
				Name: prefix + "_errors_total",
				Help: "Application errors for " + serviceName + ".",
			}, []string{"type", "env"})
			registry.MustRegister(aliasErrors)
		}

		registerRuntimeAliases(serviceName, env)
		registerConsumerMetrics(serviceName, aliasPrefix(serviceName))
	})

	serviceUp.WithLabelValues(serviceName, env).Set(1)
	healthcheckTotal.WithLabelValues(serviceName, "ok", env).Inc()
}

func aliasPrefix(serviceName string) string {
	switch serviceName {
	case "chat-api":
		return "chat"
	case "review-api":
		return "review"
	case "review-webhook":
		return "webhook"
	case "review-consumer":
		return "consumer"
	case "issue-consumer":
		return "issue"
	case "orchestrator":
		return "orchestrator"
	case "identity":
		return "identity"
	default:
		return ""
	}
}

func registerRuntimeAliases(serviceName, env string) {
	constLabels := prometheus.Labels{"service": serviceName, "env": env}

	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "service_runtime_goroutines",
		Help:        "Number of goroutines.",
		ConstLabels: constLabels,
	}, func() float64 { return float64(runtime.NumGoroutine()) }))

	registry.MustRegister(prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name:        "service_runtime_alloc_bytes",
		Help:        "Bytes allocated and in use.",
		ConstLabels: constLabels,
	}, func() float64 {
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		return float64(mem.Alloc)
	}))
}

// Handler exposes Prometheus metrics.
func Handler() http.Handler {
	return promhttp.HandlerFor(registry, promhttp.HandlerOpts{EnableOpenMetrics: true})
}

func observe(service, method, path string, status int, elapsed time.Duration) {
	statusStr := strconv.Itoa(status)

	httpRequests.WithLabelValues(service, method, path, statusStr, env).Inc()
	httpDuration.WithLabelValues(service, method, path, statusStr, env).Observe(elapsed.Seconds())

	if status >= 500 {
		errorTotal.WithLabelValues(service, "http_5xx", env).Inc()
	} else if status >= 400 {
		errorTotal.WithLabelValues(service, "http_4xx", env).Inc()
	}

	if aliasHTTPRequests != nil {
		aliasHTTPRequests.WithLabelValues(method, path, statusStr, env).Inc()
		aliasHTTPDuration.WithLabelValues(method, path, statusStr, env).Observe(elapsed.Seconds())
	}
}

func trackInFlight(service string, delta float64) {
	httpInFlight.WithLabelValues(service, env).Add(delta)
	if aliasHTTPInFlight != nil {
		aliasHTTPInFlight.WithLabelValues(env).Add(delta)
	}
}

// IncError increments service_errors_total for non-HTTP failures.
func IncError(service, errType string) {
	if errorTotal == nil {
		return
	}
	errorTotal.WithLabelValues(service, errType, env).Inc()
	if aliasErrors != nil && service == currentService {
		aliasErrors.WithLabelValues(errType, env).Inc()
	}
}
