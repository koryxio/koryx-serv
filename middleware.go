package main

import (
	"compress/gzip"
	"crypto/subtle"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Middleware type definition
type Middleware func(http.Handler) http.Handler

// Chain applies multiple middleware functions
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// LoggingMiddleware adds request logging
func LoggingMiddleware(logger *Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrapper to capture the status code
			wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start)
			logger.Access(r.Method, r.URL.Path, wrapped.statusCode, duration, r.RemoteAddr)
		})
	}
}

// responseWriter wraps ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// SecurityHeadersMiddleware adds security headers
func SecurityHeadersMiddleware() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			next.ServeHTTP(w, r)
		})
	}
}

// BlockHiddenFilesMiddleware blocks access to hidden files
func BlockHiddenFilesMiddleware(rootDir string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check whether the path contains hidden files/directories
			parts := strings.Split(filepath.Clean(r.URL.Path), string(filepath.Separator))
			for _, part := range parts {
				if strings.HasPrefix(part, ".") && part != "." && part != ".." {
					http.Error(w, "403 Forbidden", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PathTraversalMiddleware protects against path traversal
func PathTraversalMiddleware(rootDir string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Clean and normalize the path
			cleanPath := filepath.Clean(r.URL.Path)

			// Check whether it contains ..
			if strings.Contains(cleanPath, "..") {
				http.Error(w, "403 Forbidden", http.StatusForbidden)
				return
			}

			// Update the request with the clean path
			r.URL.Path = cleanPath
			next.ServeHTTP(w, r)
		})
	}
}

// BasicAuthMiddleware adds basic authentication
func BasicAuthMiddleware(config *BasicAuthConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			username, password, ok := r.BasicAuth()
			if !ok {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			// Use constant-time comparison to avoid timing attacks
			usernameMatch := subtle.ConstantTimeCompare([]byte(username), []byte(config.Username)) == 1
			passwordMatch := subtle.ConstantTimeCompare([]byte(password), []byte(config.Password)) == 1

			if !usernameMatch || !passwordMatch {
				w.Header().Set("WWW-Authenticate", `Basic realm="`+config.Realm+`"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware adds CORS support
func CORSMiddleware(config *CORSConfig) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			origin := r.Header.Get("Origin")

			// Check whether the origin is allowed
			allowed := false
			for _, allowedOrigin := range config.AllowedOrigins {
				if allowedOrigin == "*" || allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed {
				if len(config.AllowedOrigins) == 1 && config.AllowedOrigins[0] == "*" {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
				}

				if len(config.AllowedMethods) > 0 {
					w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowedMethods, ", "))
				}

				if len(config.AllowedHeaders) > 0 {
					w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowedHeaders, ", "))
				}

				if config.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}

				if config.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", strconv.Itoa(config.MaxAge))
				}
			}

			// Handle preflight
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimiter implements rate limiting
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	config   *RateLimitConfig
}

type visitor struct {
	lastSeen time.Time
	tokens   int
}

func NewRateLimiter(config *RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		config:   config,
	}

	// Periodically clean up stale visitors
	go rl.cleanupVisitors()

	return rl
}

func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	capacity := rl.config.BurstSize
	if capacity <= 0 {
		capacity = rl.config.RequestsPerIP
	}
	if capacity <= 0 {
		capacity = 1
	}

	now := time.Now()
	v, exists := rl.visitors[ip]

	if !exists {
		initialTokens := capacity - 1
		if initialTokens < 0 {
			initialTokens = 0
		}
		rl.visitors[ip] = &visitor{
			lastSeen: now,
			tokens:   initialTokens,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(v.lastSeen)
	tokensToAdd := int(elapsed.Minutes() * float64(rl.config.RequestsPerIP))
	v.tokens += tokensToAdd

	if v.tokens > capacity {
		v.tokens = capacity
	}

	v.lastSeen = now

	if v.tokens > 0 {
		v.tokens--
		return true
	}

	return false
}

// RateLimitMiddleware adds request rate limiting
func RateLimitMiddleware(limiter *RateLimiter) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil || !limiter.config.Enabled {
				next.ServeHTTP(w, r)
				return
			}

			ip := clientIP(r.RemoteAddr)

			if !limiter.allow(ip) {
				http.Error(w, "429 Too Many Requests", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// IPFilterMiddleware filters IPs based on whitelist/blacklist
func IPFilterMiddleware(whitelist, blacklist []string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := clientIP(r.RemoteAddr)

			// Check blacklist first
			for _, blocked := range blacklist {
				if ip == blocked {
					http.Error(w, "403 Forbidden", http.StatusForbidden)
					return
				}
			}

			// If whitelist exists, check whether IP is allowed
			if len(whitelist) > 0 {
				allowed := false
				for _, allowed_ip := range whitelist {
					if ip == allowed_ip {
						allowed = true
						break
					}
				}
				if !allowed {
					http.Error(w, "403 Forbidden", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return strings.Trim(remoteAddr, "[]")
	}
	return host
}

// CompressionMiddleware adds gzip compression
func CompressionMiddleware(level int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check whether the client accepts gzip
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("Content-Encoding", "gzip")

			gz, err := gzip.NewWriterLevel(w, level)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}
			defer gz.Close()

			gzw := &gzipResponseWriter{ResponseWriter: w, Writer: gz}
			next.ServeHTTP(gzw, r)
		})
	}
}

type gzipResponseWriter struct {
	http.ResponseWriter
	io.Writer
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// CustomHeadersMiddleware adds custom headers
func CustomHeadersMiddleware(headers map[string]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for key, value := range headers {
				w.Header().Set(key, value)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CacheMiddleware adds cache headers
func CacheMiddleware(maxAge int) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if maxAge > 0 {
				w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%d", maxAge))
			}
			next.ServeHTTP(w, r)
		})
	}
}
