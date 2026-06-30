package middleware

import (
	"bytes"
	"net/http"
	"sync"

	"github.com/zeromicro/go-zero/core/logx"
)

var recorderBufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// responseRecorder wraps http.ResponseWriter to capture status code and body.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	buf := recorderBufPool.Get().(*bytes.Buffer)
	buf.Reset()
	return &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK, body: buf}
}

func putResponseRecorder(r *responseRecorder) {
	if r.body != nil {
		r.body.Reset()
		recorderBufPool.Put(r.body)
	}
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	r.body.Write(b)
	return r.ResponseWriter.Write(b)
}

// Flush implements http.Flusher so that SSE streaming works through this middleware.
func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the underlying http.ResponseWriter so http.NewResponseController
// can find the http.Flusher.
func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// exemptPaths are endpoints whose responses intentionally do not use the standard data envelope.
var exemptPaths = map[string]bool{
	"/api/v1/auth/register":                  true,
	"/api/v1/auth/login":                     true,
	"/api/v1/auth/refresh":                   true,
	"/api/v1/auth/logout":                    true,
	"/api/v1/billing/webhook":                true,
	"/api/v1/weekly-reviews/generate-stream": true, // SSE endpoint
}

// jsonHasTopLevelKey reports whether data (a JSON object) contains key as a
// top-level member key. It does not allocate.
func jsonHasTopLevelKey(data []byte, key string) bool {
	depth := 0
	inString := false
	escape := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inString = !inString
			if !inString && depth == 1 {
				start := i - len(key)
				if start > 0 && data[start-1] == '"' {
					if string(data[start:i]) == key {
						j := i + 1
						for j < len(data) {
							switch data[j] {
							case ' ', '\t', '\n', '\r':
								j++
							default:
								goto checkColon
							}
						}
					checkColon:
						if j < len(data) && data[j] == ':' {
							return true
						}
					}
				}
			}
			continue
		}
		if !inString {
			switch c {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return false
}

// ResponseShapeMiddleware verifies that 2xx responses contain a "data" field
// and that non-2xx responses match the ErrorResponse shape. It logs warnings
// for non-compliant responses without blocking the request.
func ResponseShapeMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			// Skip the response recorder entirely for exempt paths (e.g. SSE
			// endpoints). Wrapping them would buffer every streamed chunk into a
			// growing bytes.Buffer for the post-handler shape check that we skip
			// anyway, wasting memory on long-lived connections and inserting an
			// extra writer layer between the SSE handler and the real socket.
			if exemptPaths[r.URL.Path] {
				next(w, r)
				return
			}

			rec := newResponseRecorder(w)
			next(rec, r)
			defer putResponseRecorder(rec)

			// Only inspect JSON responses
			contentType := rec.Header().Get("Content-Type")
			if contentType != "" && contentType != "application/json" {
				return
			}

			body := rec.body.Bytes()
			if len(body) == 0 {
				return
			}

			// Fast zero-allocation top-level key check.
			if rec.statusCode >= 200 && rec.statusCode < 300 {
				if !jsonHasTopLevelKey(body, "data") {
					logx.WithContext(r.Context()).Errorf(
						"response shape: missing 'data' field in 2xx response on %s (status %d)",
						r.URL.Path, rec.statusCode,
					)
				}
			} else {
				if !jsonHasTopLevelKey(body, "code") {
					logx.WithContext(r.Context()).Errorf(
						"response shape: missing 'code' field in error response on %s (status %d)",
						r.URL.Path, rec.statusCode,
					)
				}
				if !jsonHasTopLevelKey(body, "message") {
					logx.WithContext(r.Context()).Errorf(
						"response shape: missing 'message' field in error response on %s (status %d)",
						r.URL.Path, rec.statusCode,
					)
				}
			}
		}
	}
}
