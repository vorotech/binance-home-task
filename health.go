package main

import (
	"time"

	"github.com/heptiolabs/healthcheck"
)

func healtcheck() healthcheck.Handler {
	health := healthcheck.NewHandler()

	// App is not ready if can't resolve the upstream dependency in DNS.
	health.AddReadinessCheck(
		"upstream-dep-dns",
		healthcheck.DNSResolveCheck(apiBaseUrl, 50*time.Millisecond))

	// Add a liveness check against the API ping endpoint
	// The check fails if the response times out or returns a non-200 status code.
	upstreamURL := apiBaseUrl + "/api/v3/ping"
	health.AddLivenessCheck(
		"upstream-dep-http",
		healthcheck.HTTPGetCheck(upstreamURL, 500*time.Millisecond))

	// Our app is not happy if we've got more than 100 goroutines running.
	health.AddLivenessCheck("goroutine-threshold", healthcheck.GoroutineCountCheck(100))

	return health
}
