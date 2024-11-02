package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	httpRequestsTotal := prometheus.NewCounter(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "Total number of HTTP requests",
	})
	prometheus.MustRegister(httpRequestsTotal)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		httpRequestsTotal.Inc()
		w.Write([]byte("Hello, World!"))
	})

	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":8080", nil)
}
