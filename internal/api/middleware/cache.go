package middleware

import (
	"bytes"
	"io"
	"net/http"
	"strings"

	"annet-oil/internal/cache"
)

type cacheResponseWriter struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *cacheResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func CacheMiddleware(c *cache.MemoryCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !isCacheable(r) {
				next.ServeHTTP(w, r)
				return
			}

			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewReader(body))

			cacheKey := cache.HashRequest(struct {
				Method string
				Path   string
				Body   string
			}{r.Method, r.URL.Path, string(body)})

			if cached, ok := c.Get(cacheKey); ok {
				w.Header().Set("X-Cache", "HIT")
				w.Header().Set("Content-Type", "application/json")
				w.Write(cached)
				return
			}

			crw := &cacheResponseWriter{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				status:         http.StatusOK,
			}

			next.ServeHTTP(crw, r)

			if crw.status >= 200 && crw.status < 300 {
				c.Set(cacheKey, crw.body.Bytes())
				w.Header().Set("X-Cache", "MISS")
			}
		})
	}
}

func isCacheable(r *http.Request) bool {
	path := r.URL.Path

	cacheablePaths := []string{"/gen", "/diff", "/routing"}
	for _, p := range cacheablePaths {
		if strings.Contains(path, p) {
			return true
		}
	}

	return false
}

func InvalidateCacheOnDeploy(c *cache.MemoryCache) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)

			if strings.Contains(r.URL.Path, "/deploy") && r.Method == http.MethodPost {
				c.InvalidateAll()
			}
		})
	}
}
