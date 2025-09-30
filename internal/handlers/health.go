package handlers

import (
	"net/http"
	"time"
)

// Healthz is the liveness probe endpoint.
//
// Purpose:
// - Answers the question: "Is the process alive and able to serve HTTP?"
// - Used by orchestrators (Kubernetes, systemd, load balancers) to decide when to
//   restart a crashed or wedged process.
//
// Behavior:
// - Should be extremely lightweight and never block on external dependencies
//   (DB, Redis, network) — if it does, it can create cascading failures.
// - Returns 200 OK when the process is running; only returns non-200 in truly
//   fatal states (e.g., initialization failed and app cannot recover).
// - Accepts GET/HEAD; response is intentionally tiny.
// - Responses are marked as non-cacheable.
func Healthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}

// Readyz is the readiness probe endpoint.
//
// Purpose:
// - Answers the question: "Can this instance accept traffic right now?"
// - Used by orchestrators and load balancers to include/exclude the instance
//   from the serving pool without restarting it.
//
// Behavior:
// - May check critical dependencies (e.g., DB connectivity, Redis availability,
//   warm caches, follower/scraper freshness). If any critical dependency is not
//   ready, this endpoint should return 503 Service Unavailable.
// - Returns 200 OK when the instance is able to serve user requests in a
//   meaningful way.
// - Responses are marked as non-cacheable.
//
// Current implementation:
// - In this starter skeleton we assume readiness immediately and return 200.
//   Replace the stub with real checks as subsystems are added.
func Readyz(w http.ResponseWriter, r *http.Request) {
	// In the base skeleton, we consider the service ready immediately.
	// Later this can check DB/Redis/scraper/indexer readiness
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ready"}`))
}
