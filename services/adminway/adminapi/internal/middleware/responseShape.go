package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/zeromicro/go-zero/rest"
)

type bodyWrapWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyWrapWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
}

func (w *bodyWrapWriter) WriteHeader(statusCode int) {
	w.ResponseWriter.WriteHeader(statusCode)
}

func ResponseShapeMiddleware() rest.Middleware {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			buf := &bytes.Buffer{}
			bww := &bodyWrapWriter{ResponseWriter: w, body: buf}
			next(bww, r)

			resp := buf.Bytes()
			if len(resp) == 0 {
				return
			}

			var raw json.RawMessage
			if err := json.Unmarshal(resp, &raw); err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write(resp)
				return
			}

			var obj map[string]json.RawMessage
			if err := json.Unmarshal(resp, &obj); err != nil {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write(resp)
				return
			}

			if _, hasData := obj["data"]; !hasData {
				envelope := map[string]json.RawMessage{
					"data": raw,
				}
				enveloped, _ := json.Marshal(envelope)
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write(enveloped)
				return
			}

			if _, ok := obj["data"]; ok {
				w.Header().Set("Content-Type", "application/json; charset=utf-8")
				w.Write(resp)
			}
		}
	}
}

func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(r.Body)
	r.Body.Close()
	return body, err
}
