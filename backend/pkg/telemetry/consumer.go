package telemetry

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	currentService      string
	consumerAliasPrefix string

	consumerMessages         *prometheus.CounterVec
	consumerSkipped          *prometheus.CounterVec
	consumerProcessingDur    *prometheus.HistogramVec
	consumerKafkaFetchErrors *prometheus.CounterVec
	consumerKafkaCommitErrs  *prometheus.CounterVec
	consumerKafkaRetries     *prometheus.CounterVec
	consumerDedupSize        *prometheus.GaugeVec
	consumerInFlight         *prometheus.GaugeVec
	consumerLLMRequests      *prometheus.CounterVec
	consumerLLMDuration      *prometheus.HistogramVec
	consumerOutcomes         *prometheus.CounterVec

	aliasConsumerMessages      *prometheus.CounterVec
	aliasConsumerSkipped       *prometheus.CounterVec
	aliasConsumerProcessingDur *prometheus.HistogramVec
	aliasConsumerKafkaFetchErr *prometheus.CounterVec
	aliasConsumerKafkaCommit   *prometheus.CounterVec
	aliasConsumerKafkaRetries  *prometheus.CounterVec
	aliasConsumerDedupSize     *prometheus.GaugeVec
	aliasConsumerInFlight      *prometheus.GaugeVec
	aliasConsumerLLMRequests   *prometheus.CounterVec
	aliasConsumerLLMDuration   *prometheus.HistogramVec
	aliasConsumerOutcomes      *prometheus.CounterVec
)

var consumerDurationBuckets = []float64{0.5, 1, 2, 5, 10, 30, 60, 120, 300, 600}

var consumerErrorTypes = []string{
	"kafka_fetch", "kafka_commit", "llm",
	"generation_permanent", "generation_transient",
	"push_permanent", "push_transient",
	"pr_permanent", "pr_transient",
	"comment_permanent", "comment_transient",
}

func registerConsumerMetrics(serviceName, alias string) {
	currentService = serviceName
	consumerAliasPrefix = alias

	consumerMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_messages_total",
		Help: "Kafka messages processed by a consumer worker.",
	}, []string{"service", "result", "env"})
	consumerSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_messages_skipped_total",
		Help: "Kafka messages intentionally skipped by a consumer worker.",
	}, []string{"service", "reason", "env"})
	consumerProcessingDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "service_consumer_processing_duration_seconds",
		Help:    "End-to-end message processing latency in seconds.",
		Buckets: consumerDurationBuckets,
	}, []string{"service", "result", "env"})
	consumerKafkaFetchErrors = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_kafka_fetch_errors_total",
		Help: "Kafka fetch failures in a consumer worker.",
	}, []string{"service", "env"})
	consumerKafkaCommitErrs = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_kafka_commit_errors_total",
		Help: "Kafka commit failures in a consumer worker.",
	}, []string{"service", "env"})
	consumerKafkaRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_kafka_retries_total",
		Help: "Transient message retries in a consumer worker.",
	}, []string{"service", "env"})
	consumerDedupSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "service_consumer_dedup_cache_size",
		Help: "Current deduplication cache size.",
	}, []string{"service", "env"})
	consumerInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "service_consumer_in_flight",
		Help: "Messages currently being processed.",
	}, []string{"service", "env"})
	consumerLLMRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_llm_requests_total",
		Help: "LLM requests made by a consumer worker.",
	}, []string{"service", "status", "env"})
	consumerLLMDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "service_consumer_llm_duration_seconds",
		Help:    "LLM request latency in seconds.",
		Buckets: consumerDurationBuckets,
	}, []string{"service", "env"})
	consumerOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "service_consumer_outcomes_total",
		Help: "Successful pipeline outcomes from a consumer worker.",
	}, []string{"service", "outcome", "env"})

	registry.MustRegister(
		consumerMessages,
		consumerSkipped,
		consumerProcessingDur,
		consumerKafkaFetchErrors,
		consumerKafkaCommitErrs,
		consumerKafkaRetries,
		consumerDedupSize,
		consumerInFlight,
		consumerLLMRequests,
		consumerLLMDuration,
		consumerOutcomes,
	)

	if alias == "" {
		primeConsumerMetrics(serviceName, alias)
		return
	}

	aliasConsumerMessages = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_messages_total",
		Help: "Messages processed by " + serviceName + ".",
	}, []string{"result", "env"})
	aliasConsumerSkipped = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_messages_skipped_total",
		Help: "Messages skipped by " + serviceName + ".",
	}, []string{"reason", "env"})
	aliasConsumerProcessingDur = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    alias + "_processing_duration_seconds",
		Help:    "Message processing latency for " + serviceName + ".",
		Buckets: consumerDurationBuckets,
	}, []string{"result", "env"})
	aliasConsumerKafkaFetchErr = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_kafka_fetch_errors_total",
		Help: "Kafka fetch failures for " + serviceName + ".",
	}, []string{"env"})
	aliasConsumerKafkaCommit = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_kafka_commit_errors_total",
		Help: "Kafka commit failures for " + serviceName + ".",
	}, []string{"env"})
	aliasConsumerKafkaRetries = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_kafka_retries_total",
		Help: "Transient message retries for " + serviceName + ".",
	}, []string{"env"})
	aliasConsumerDedupSize = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: alias + "_dedup_cache_size",
		Help: "Deduplication cache size for " + serviceName + ".",
	}, []string{"env"})
	aliasConsumerInFlight = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: alias + "_in_flight",
		Help: "In-flight messages for " + serviceName + ".",
	}, []string{"env"})
	aliasConsumerLLMRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_llm_requests_total",
		Help: "LLM requests for " + serviceName + ".",
	}, []string{"status", "env"})
	aliasConsumerLLMDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    alias + "_llm_duration_seconds",
		Help:    "LLM latency for " + serviceName + ".",
		Buckets: consumerDurationBuckets,
	}, []string{"env"})
	aliasConsumerOutcomes = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: alias + "_outcomes_total",
		Help: "Pipeline outcomes for " + serviceName + ".",
	}, []string{"outcome", "env"})

	registry.MustRegister(
		aliasConsumerMessages,
		aliasConsumerSkipped,
		aliasConsumerProcessingDur,
		aliasConsumerKafkaFetchErr,
		aliasConsumerKafkaCommit,
		aliasConsumerKafkaRetries,
		aliasConsumerDedupSize,
		aliasConsumerInFlight,
		aliasConsumerLLMRequests,
		aliasConsumerLLMDuration,
		aliasConsumerOutcomes,
	)

	primeConsumerMetrics(serviceName, alias)
}

func primeConsumerMetrics(serviceName, alias string) {
	for _, result := range []string{"done", "skip", "transient"} {
		consumerMessages.WithLabelValues(serviceName, result, env)
		consumerProcessingDur.WithLabelValues(serviceName, result, env).Observe(0)
		if aliasConsumerMessages != nil {
			aliasConsumerMessages.WithLabelValues(result, env)
			aliasConsumerProcessingDur.WithLabelValues(result, env).Observe(0)
		}
	}

	for _, reason := range []string{
		"decode_error", "non_issue_event", "unhandled_action", "not_assigned",
		"missing_ref", "dedup", "not_onboarded", "generation_permanent",
		"push_permanent", "comment_permanent",
	} {
		consumerSkipped.WithLabelValues(serviceName, reason, env)
		if aliasConsumerSkipped != nil {
			aliasConsumerSkipped.WithLabelValues(reason, env)
		}
	}

	for _, status := range []string{"success", "error", "repair"} {
		consumerLLMRequests.WithLabelValues(serviceName, status, env)
		if aliasConsumerLLMRequests != nil {
			aliasConsumerLLMRequests.WithLabelValues(status, env)
		}
	}

	for _, outcome := range []string{"branch_pushed", "pr_created", "comment_posted", "not_onboarded"} {
		consumerOutcomes.WithLabelValues(serviceName, outcome, env)
		if aliasConsumerOutcomes != nil {
			aliasConsumerOutcomes.WithLabelValues(outcome, env)
		}
	}

	consumerKafkaFetchErrors.WithLabelValues(serviceName, env)
	consumerKafkaCommitErrs.WithLabelValues(serviceName, env)
	consumerKafkaRetries.WithLabelValues(serviceName, env)
	consumerDedupSize.WithLabelValues(serviceName, env).Set(0)
	consumerInFlight.WithLabelValues(serviceName, env).Set(0)
	consumerLLMDuration.WithLabelValues(serviceName, env).Observe(0)

	if alias != "" {
		aliasConsumerKafkaFetchErr.WithLabelValues(env)
		aliasConsumerKafkaCommit.WithLabelValues(env)
		aliasConsumerKafkaRetries.WithLabelValues(env)
		aliasConsumerDedupSize.WithLabelValues(env).Set(0)
		aliasConsumerInFlight.WithLabelValues(env).Set(0)
		aliasConsumerLLMDuration.WithLabelValues(env).Observe(0)
	}

	for _, errType := range consumerErrorTypes {
		errorTotal.WithLabelValues(serviceName, errType, env)
		if aliasErrors != nil {
			aliasErrors.WithLabelValues(errType, env)
		}
	}
}

// ObserveConsumerProcess records message processing result and latency.
func ObserveConsumerProcess(result string, skipReason string, elapsed time.Duration) {
	if consumerMessages == nil {
		return
	}
	consumerMessages.WithLabelValues(currentService, result, env).Inc()
	consumerProcessingDur.WithLabelValues(currentService, result, env).Observe(elapsed.Seconds())
	if aliasConsumerMessages != nil {
		aliasConsumerMessages.WithLabelValues(result, env).Inc()
		aliasConsumerProcessingDur.WithLabelValues(result, env).Observe(elapsed.Seconds())
	}
	if result == "skip" && skipReason != "" {
		IncConsumerSkipped(skipReason)
	}
}

// IncConsumerSkipped increments the skip counter for a specific reason.
func IncConsumerSkipped(reason string) {
	if consumerSkipped == nil {
		return
	}
	consumerSkipped.WithLabelValues(currentService, reason, env).Inc()
	if aliasConsumerSkipped != nil {
		aliasConsumerSkipped.WithLabelValues(reason, env).Inc()
	}
}

// IncConsumerKafkaFetchError records a Kafka fetch failure.
func IncConsumerKafkaFetchError() {
	if consumerKafkaFetchErrors == nil {
		return
	}
	consumerKafkaFetchErrors.WithLabelValues(currentService, env).Inc()
	if aliasConsumerKafkaFetchErr != nil {
		aliasConsumerKafkaFetchErr.WithLabelValues(env).Inc()
	}
	IncError(currentService, "kafka_fetch")
}

// IncConsumerKafkaCommitError records a Kafka commit failure.
func IncConsumerKafkaCommitError() {
	if consumerKafkaCommitErrs == nil {
		return
	}
	consumerKafkaCommitErrs.WithLabelValues(currentService, env).Inc()
	if aliasConsumerKafkaCommit != nil {
		aliasConsumerKafkaCommit.WithLabelValues(env).Inc()
	}
	IncError(currentService, "kafka_commit")
}

// IncConsumerKafkaRetry records a transient message retry.
func IncConsumerKafkaRetry() {
	if consumerKafkaRetries == nil {
		return
	}
	consumerKafkaRetries.WithLabelValues(currentService, env).Inc()
	if aliasConsumerKafkaRetries != nil {
		aliasConsumerKafkaRetries.WithLabelValues(env).Inc()
	}
}

// TrackConsumerInFlight adjusts the in-flight message gauge.
func TrackConsumerInFlight(delta float64) {
	if consumerInFlight == nil {
		return
	}
	consumerInFlight.WithLabelValues(currentService, env).Add(delta)
	if aliasConsumerInFlight != nil {
		aliasConsumerInFlight.WithLabelValues(env).Add(delta)
	}
}

// SetConsumerDedupSize sets the deduplication cache size gauge.
func SetConsumerDedupSize(size int) {
	if consumerDedupSize == nil {
		return
	}
	val := float64(size)
	consumerDedupSize.WithLabelValues(currentService, env).Set(val)
	if aliasConsumerDedupSize != nil {
		aliasConsumerDedupSize.WithLabelValues(env).Set(val)
	}
}

// ObserveConsumerLLM records an LLM request outcome and latency.
func ObserveConsumerLLM(status string, elapsed time.Duration) {
	if consumerLLMRequests == nil {
		return
	}
	consumerLLMRequests.WithLabelValues(currentService, status, env).Inc()
	if elapsed > 0 {
		consumerLLMDuration.WithLabelValues(currentService, env).Observe(elapsed.Seconds())
	}
	if aliasConsumerLLMRequests != nil {
		aliasConsumerLLMRequests.WithLabelValues(status, env).Inc()
		if elapsed > 0 {
			aliasConsumerLLMDuration.WithLabelValues(env).Observe(elapsed.Seconds())
		}
	}
	if status == "error" {
		IncError(currentService, "llm")
	}
}

// IncConsumerOutcome records a successful pipeline outcome.
func IncConsumerOutcome(outcome string) {
	if consumerOutcomes == nil {
		return
	}
	consumerOutcomes.WithLabelValues(currentService, outcome, env).Inc()
	if aliasConsumerOutcomes != nil {
		aliasConsumerOutcomes.WithLabelValues(outcome, env).Inc()
	}
}
