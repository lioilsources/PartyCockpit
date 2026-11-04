package api

// Per-IP token bucket for what anyone in the LAN can call without a token:
// guest writes, joining, creating a party, the recap zip (D19). The cockpit
// is not limited: it has a token and one operator.

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	writeRate  = 3.0  // tokens per second (30 per 10 s)
	writeBurst = 30.0 // a phone may fire a quick burst (reactions, votes)
)

type bucket struct {
	tokens float64
	at     time.Time
}

type limiter struct {
	mu      sync.Mutex
	now     func() time.Time
	rate    float64
	burst   float64
	buckets map[string]*bucket
	swept   time.Time
}

func newLimiter(now func() time.Time, rate, burst float64) *limiter {
	return &limiter{now: now, rate: rate, burst: burst, buckets: map[string]*bucket{}}
}

// allow takes cost tokens from key's bucket; false = over the limit.
func (l *limiter) allow(key string, cost float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.swept) > 10*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.at) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}
	b := l.buckets[key]
	if b == nil {
		b = &bucket{tokens: l.burst, at: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.at).Seconds()*l.rate)
	b.at = now
	if b.tokens < cost {
		return false
	}
	b.tokens -= cost
	return true
}

// clientIP is the remote address; X-Forwarded-For only behind a trusted proxy
// (PC_TRUST_PROXY=1), otherwise any phone could pick its own "IP".
func (s *Server) clientIP(r *http.Request) string {
	if s.Cfg.TrustProxy {
		if f := r.Header.Get("X-Forwarded-For"); f != "" {
			return strings.TrimSpace(strings.Split(f, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// allowIP answers 429 itself when the caller's IP is over the limit.
func (s *Server) allowIP(w http.ResponseWriter, r *http.Request, cost float64) bool {
	if s.limits.allow(s.clientIP(r), cost) {
		return true
	}
	writeErr(w, 429, "moc rychle, chvilku počkej")
	return false
}
