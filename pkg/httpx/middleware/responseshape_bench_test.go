package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var benchBody = []byte(`{"data":{"articles":[{"id":"1","title":"T"},{"id":"2","title":"U"}],"total":2}}`)

func BenchmarkResponseShapeMiddleware(b *testing.B) {
	handler := ResponseShapeMiddleware(nil)(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(benchBody)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)

	b.ReportAllocs()

	for b.Loop() {
		rec := httptest.NewRecorder()
		handler(rec, req)
	}
}

func BenchmarkJSONHasTopLevelKey(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = jsonHasTopLevelKey(benchBody, "data")
	}
}

func BenchmarkOldUnmarshal(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		var parsed map[string]interface{}
		_ = fmt.Sprintf("%v", parsed) // prevent optimization
	}
}
