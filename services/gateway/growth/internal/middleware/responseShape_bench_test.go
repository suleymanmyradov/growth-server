package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

var benchBody = []byte(`{"data":{"articles":[{"id":"1","title":"T"},{"id":"2","title":"U"}],"total":2}}`)

func BenchmarkResponseShapeMiddleware(b *testing.B) {
	handler := ResponseShapeMiddleware()(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(benchBody)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/articles", nil)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rec := httptest.NewRecorder()
		handler(rec, req)
	}
}

func BenchmarkJSONHasTopLevelKey(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = jsonHasTopLevelKey(benchBody, "data")
	}
}

func BenchmarkOldUnmarshal(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var parsed map[string]interface{}
		_ = fmt.Sprintf("%v", parsed) // prevent optimization
	}
}
