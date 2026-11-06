package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Define metrics
var (
	httpRequestTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_request_total",
			Help: "Total # of HTTP receieved",
		},
		[]string{"method", "status"},
	)

	httpRequestLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_latency_seconds",
			Help:    "Latency of requests in s",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"handler"},
	)
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Register collectors with default registers
func init() {
	prometheus.MustRegister(httpRequestTotal)
	prometheus.MustRegister(httpRequestLatency)
}

// Collection middleware
func instrumentedHandler(fn http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// grab status
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		fn(rw, r)

		// grab latency
		latency := time.Since(start).Seconds()
		httpRequestLatency.WithLabelValues("main").Observe(latency)

		// count
		httpRequestTotal.WithLabelValues(r.Method, fmt.Sprintf("%d", rw.statusCode)).Inc()
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	// Simulate latency
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)

	// Simulate random failures (~20% of requests)
	if rand.Intn(100) < 20 {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, "Hello, World!")
}

func main() {
	http.HandleFunc("/", instrumentedHandler(helloHandler))
	http.Handle("/metrics", promhttp.Handler()) // The metrics endpoint actually allows connectivity with prometheus

	fmt.Println("Server listening on :8080")

	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
