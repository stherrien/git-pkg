package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type visitor struct {
	tokens    float64
	lastSeen  time.Time
	maxTokens float64
	rate      float64 // tokens per second
}

// Limiter provides per-IP rate limiting using a token bucket algorithm.
type Limiter struct {
	mu        sync.Mutex
	visitors  map[string]*visitor
	rate      float64 // tokens per second
	burst     int     // max tokens
	cleanupAt time.Time
}

// New creates a Limiter that allows `rate` requests per second with a burst of `burst`.
func New(rate float64, burst int) *Limiter {
	l := &Limiter{
		visitors: make(map[string]*visitor),
		rate:     rate,
		burst:    burst,
	}
	go l.cleanupLoop()
	return l
}

// Middleware wraps an http.Handler with rate limiting.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !l.allow(ip) {
			retryAfter := 1.0 / l.rate
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter)+1))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	v, exists := l.visitors[ip]
	now := time.Now()

	if !exists {
		l.visitors[ip] = &visitor{
			tokens:    float64(l.burst) - 1,
			lastSeen:  now,
			maxTokens: float64(l.burst),
			rate:      l.rate,
		}
		return true
	}

	// Refill tokens based on elapsed time
	elapsed := now.Sub(v.lastSeen).Seconds()
	v.tokens += elapsed * v.rate
	if v.tokens > v.maxTokens {
		v.tokens = v.maxTokens
	}
	v.lastSeen = now

	if v.tokens < 1 {
		return false
	}
	v.tokens--
	return true
}

func (l *Limiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		l.mu.Lock()
		cutoff := time.Now().Add(-10 * time.Minute)
		for ip, v := range l.visitors {
			if v.lastSeen.Before(cutoff) {
				delete(l.visitors, ip)
			}
		}
		l.mu.Unlock()
	}
}

func extractIP(r *http.Request) string {
	// Trust Fly.io's forwarded header
	if forwarded := r.Header.Get("Fly-Client-IP"); forwarded != "" {
		return forwarded
	}
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		return forwarded
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
