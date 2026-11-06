package main

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
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

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Hello, World!")
}

func main() {
	http.HandleFunc("/", helloHandler)

	fmt.Println("Server listening on :8080")

	if err := ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
