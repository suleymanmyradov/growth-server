package ai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

// normalizeArrayWrappedErrorBody detects Google's Gemini OpenAI-compat
// endpoint error responses that are wrapped in a JSON array (e.g.
// `[{"error":{"code":400,"message":"..."}}]`) instead of the standard
// object form (`{"error":{...}}`). When detected, it unwraps the first
// element of the array so the go-openai client's ErrorResponse
// unmarshaling succeeds and the real error message/status is surfaced.
//
// Without this, the go-openai client fails with
// "json: cannot unmarshal array into Go value of type openai.ErrorResponse",
// which masks the real error (e.g. "Function call is missing a
// thought_signature") and hides the HTTP status code from retry logic
// (isRetryable can't see the code, so it defaults to retryable=true and
// retries 400s that should not be retried).
//
// Returns the (possibly rewritten) body and whether it was changed.
// It only unwraps arrays whose first element is an object containing an
// "error" key — other arrays (e.g. legitimate array responses) are passed
// through unchanged.
func normalizeArrayWrappedErrorBody(body []byte) ([]byte, bool) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return body, false
	}

	var arr []json.RawMessage
	if err := json.Unmarshal(trimmed, &arr); err != nil {
		return body, false
	}
	if len(arr) == 0 {
		return body, false
	}

	// Check that the first element is an object with an "error" key.
	var first map[string]json.RawMessage
	if err := json.Unmarshal(arr[0], &first); err != nil {
		return body, false
	}
	if _, ok := first["error"]; !ok {
		return body, false
	}

	// Unwrap: return just the first element as the body.
	return bytes.TrimSpace(arr[0]), true
}

// normalizeErrorResponse checks if an HTTP error response (status >= 400)
// has an array-wrapped error body and, if so, rewrites the body to the
// unwrapped object form. This is applied at the transport layer so the
// go-openai client sees a standard object-form error body.
func normalizeErrorResponse(resp *http.Response) *http.Response {
	if resp.StatusCode < 400 || resp.Body == nil {
		return resp
	}

	ct := resp.Header.Get("Content-Type")
	if !isJSONContentType(ct) {
		return resp
	}

	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		// Can't read body; restore original (closed) body.
		resp.Body = io.NopCloser(bytes.NewReader(nil))
		return resp
	}

	normalized, changed := normalizeArrayWrappedErrorBody(body)
	if !changed {
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp
	}

	resp.Body = io.NopCloser(bytes.NewReader(normalized))
	resp.ContentLength = int64(len(normalized))
	return resp
}
