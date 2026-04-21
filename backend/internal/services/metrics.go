package services

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all custom Prometheus metrics for the application
type Metrics struct {
	// WebSocket metrics
	WebSocketConnections prometheus.Gauge
	WebSocketMessages    *prometheus.CounterVec

	// Chat metrics
	ChatRequests       prometheus.Counter
	ChatRequestLatency prometheus.Histogram
	ChatErrors         *prometheus.CounterVec

	// Connection manager reference for dynamic metrics
	connManager *ConnectionManager
}

var globalMetrics *Metrics

// InitMetrics initializes the Prometheus metrics
func InitMetrics(connManager *ConnectionManager) *Metrics {
	metrics := &Metrics{
		connManager: connManager,

		// WebSocket active connections (gauge - can go up and down)
		WebSocketConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "orchid_websocket_connections_active",
			Help: "Number of active WebSocket connections",
		}),

		// WebSocket messages by type (counter - only goes up)
		WebSocketMessages: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "orchid_websocket_messages_total",
			Help: "Total number of WebSocket messages by type",
		}, []string{"type", "direction"}), // direction: "inbound" or "outbound"

		// Chat requests counter
		ChatRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "orchid_chat_requests_total",
			Help: "Total number of chat requests processed",
		}),

		// Chat request latency histogram
		ChatRequestLatency: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "orchid_chat_request_duration_seconds",
			Help:    "Chat request latency in seconds",
			Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60, 120}, // up to 2 minutes for LLM responses
		}),

		// Chat errors by type
		ChatErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "orchid_chat_errors_total",
			Help: "Total number of chat errors by type",
		}, []string{"error_type"}),
	}

	// Register a collector that updates WebSocket connections from ConnectionManager
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "orchid_websocket_connections_current",
			Help: "Current number of active WebSocket connections (from connection manager)",
		},
		func() float64 {
			if connManager != nil {
				return float64(connManager.Count())
			}
			return 0
		},
	))

	globalMetrics = metrics
	return metrics
}

// GetMetrics returns the global metrics instance
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordWebSocketConnect records a new WebSocket connection
func (m *Metrics) RecordWebSocketConnect() {
	m.WebSocketConnections.Inc()
}

// RecordWebSocketDisconnect records a WebSocket disconnection
func (m *Metrics) RecordWebSocketDisconnect() {
	m.WebSocketConnections.Dec()
}

// RecordWebSocketMessage records a WebSocket message
func (m *Metrics) RecordWebSocketMessage(msgType, direction string) {
	m.WebSocketMessages.WithLabelValues(msgType, direction).Inc()
}

// RecordChatRequest records a chat request
func (m *Metrics) RecordChatRequest() {
	m.ChatRequests.Inc()
}

// RecordChatLatency records chat request latency
func (m *Metrics) RecordChatLatency(seconds float64) {
	m.ChatRequestLatency.Observe(seconds)
}

// RecordChatError records a chat error
func (m *Metrics) RecordChatError(errorType string) {
	m.ChatErrors.WithLabelValues(errorType).Inc()
}

