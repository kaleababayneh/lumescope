package server

import (
	"log"
	"net/http"
	"strings"
	"time"

	"lumescope/internal/config"
	"lumescope/internal/db"
	"lumescope/internal/handlers"
)

// NewRouter builds the HTTP router using only net/http ServeMux and stdlib middleware.
func NewRouter(cfg config.Config, pool *db.Pool, syncTrigger handlers.SyncTrigger) http.Handler {
	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("/healthz", handlers.Healthz)
	mux.HandleFunc("/readyz", handlers.Readyz)

	// Optional metrics stub (no third-party dependency)
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("# metrics disabled (no third-party deps)\n"))
	})

	// Actions list (exact path)
	mux.HandleFunc("/v1/actions", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/actions" && r.URL.Path != "/v1/actions/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		handlers.ListActions(pool)(w, r)
	})

	// Actions detail: /v1/actions/{id}
	mux.HandleFunc("/v1/actions/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/v1/actions/")
		if id == "" || strings.Contains(id, "/") {
			http.NotFound(w, r)
			return
		}
		// Delegate to handler; it will parse id from path as well.
		handlers.GetAction(pool)(w, r)
	})

	// Conditionally register sync endpoint (disabled by default)
	if cfg.EnableSyncEndpoint {
		mux.HandleFunc("/v1/supernodes/sync", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				w.Header().Set("Allow", "POST, OPTIONS")
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}
			handlers.TriggerSupernodeSync(syncTrigger)(w, r)
		})
	}

	mux.HandleFunc("/v1/supernodes/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		handlers.ListSupernodesMetrics(pool)(w, r)
	})

	mux.HandleFunc("/v1/supernodes/stats", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		handlers.GetSupernodeStats(pool)(w, r)
	})

	// Supernode detail metrics: /v1/supernodes/{id}/metrics
	mux.HandleFunc("/v1/supernodes/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		// Check if path ends with /metrics
		if strings.HasSuffix(r.URL.Path, "/metrics") {
			handlers.GetSupernodeMetrics(pool)(w, r)
			return
		}
		// If not a metrics request, return 404
		http.NotFound(w, r)
	})

	mux.HandleFunc("/v1/supernodes/unavailable", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		handlers.ListUnavailableSupernodes(pool)(w, r)
	})

	mux.HandleFunc("/v1/version/matrix", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		handlers.VersionMatrix(pool)(w, r)
	})

	// OpenAPI spec endpoint
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		http.ServeFile(w, r, "docs/openapi.json")
	})

	// Swagger UI documentation
	mux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<!DOCTYPE html>
<html lang="en">
<head>
	 <meta charset="UTF-8">
	 <title>LumeScope API Documentation</title>
	 <link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui.css" />
	 <style>
	   html { box-sizing: border-box; overflow: -moz-scrollbars-vertical; overflow-y: scroll; }
	   *, *:before, *:after { box-sizing: inherit; }
	   body { margin: 0; padding: 0; }
	 </style>
</head>
<body>
	 <div id="swagger-ui"></div>
	 <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
	 <script src="https://cdn.jsdelivr.net/npm/swagger-ui-dist@5/swagger-ui-standalone-preset.js"></script>
	 <script>
	   window.onload = function() {
	     window.ui = SwaggerUIBundle({
	       url: "/openapi.json",
	       dom_id: '#swagger-ui',
	       deepLinking: true,
	       presets: [
	         SwaggerUIBundle.presets.apis,
	         SwaggerUIStandalonePreset
	       ],
	       plugins: [
	         SwaggerUIBundle.plugins.DownloadUrl
	       ],
	       layout: "StandaloneLayout"
	     });
	   };
	 </script>
</body>
</html>`))
	})

	// Fallback 404 handler for any other path
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not_found"}`))
	})

	// Wrap mux with stdlib middlewares
	var h http.Handler = mux
	h = withServerHeader(h)
	h = withDefaultCacheControl(h)
	h = withDateHeader(h)
	h = withCORS(cfg, h)
	h = withRecover(h)
	h = http.TimeoutHandler(h, cfg.RequestTimeout, "request timeout\n")

	return h
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, OPTIONS")
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func withServerHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "LumeScope/preview")
		next.ServeHTTP(w, r)
	})
}

func withDefaultCacheControl(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Default for JSON endpoints; handlers may override.
		w.Header().Set("Cache-Control", "public, max-age=30")
		next.ServeHTTP(w, r)
	})
}

func withDateHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		next.ServeHTTP(w, r)
	})
}

// withCORS implements minimal CORS using stdlib only.
func withCORS(cfg config.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := ""
		for _, o := range cfg.AllowOrigins {
			if o == "*" || o == origin {
				allowed = o
				break
			}
		}
		if allowed == "*" || (allowed != "" && origin != "") {
			if allowed == "*" {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			} else {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, If-None-Match, If-Modified-Since")
			w.Header().Set("Access-Control-Expose-Headers", "ETag, Last-Modified")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func withRecover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, `{"error":"internal_error"}`, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
