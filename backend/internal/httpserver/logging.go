package httpserver

import (
	"net/http"
	"strings"
	"time"
)

const redactedValue = "[REDACTED]"

var sensitiveHeaders = map[string]struct{}{
	"authorization":   {},
	"cookie":          {},
	"set-cookie":      {},
	"x-csrf-token":    {},
	"x-session-token": {},
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(body []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}

	written, err := r.ResponseWriter.Write(body)
	r.bytes += written
	return written, err
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &statusRecorder{ResponseWriter: w}

		next.ServeHTTP(recorder, r)

		status := recorder.status
		if status == 0 {
			status = http.StatusOK
		}

		s.logger.Info(
			"http request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"bytes", recorder.bytes,
			"duration_ms", time.Since(started).Milliseconds(),
			"request_id", redactedHeaderValue(r.Header, "X-Request-ID"),
			"user_agent", r.UserAgent(),
			"remote_addr", r.RemoteAddr,
		)
	})
}

func redactedHeaderValue(headers http.Header, name string) string {
	if isSensitiveHeader(name) {
		return redactedValue
	}
	return headers.Get(name)
}

func redactHeaders(headers http.Header) map[string][]string {
	redacted := make(map[string][]string, len(headers))
	for name, values := range headers {
		if isSensitiveHeader(name) {
			redacted[name] = []string{redactedValue}
			continue
		}

		copied := make([]string, len(values))
		copy(copied, values)
		redacted[name] = copied
	}
	return redacted
}

func isSensitiveHeader(name string) bool {
	_, ok := sensitiveHeaders[strings.ToLower(name)]
	return ok
}
