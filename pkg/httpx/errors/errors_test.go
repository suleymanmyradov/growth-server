package errors

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGrpcToHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		grpcCode codes.Code
		httpCode int
	}{
		{"OK", codes.OK, http.StatusOK},
		{"Canceled", codes.Canceled, http.StatusRequestTimeout},
		{"Unknown", codes.Unknown, http.StatusInternalServerError},
		{"InvalidArgument", codes.InvalidArgument, http.StatusBadRequest},
		{"DeadlineExceeded", codes.DeadlineExceeded, http.StatusGatewayTimeout},
		{"NotFound", codes.NotFound, http.StatusNotFound},
		{"AlreadyExists", codes.AlreadyExists, http.StatusConflict},
		{"PermissionDenied", codes.PermissionDenied, http.StatusForbidden},
		{"Unauthenticated", codes.Unauthenticated, http.StatusUnauthorized},
		{"ResourceExhausted", codes.ResourceExhausted, http.StatusTooManyRequests},
		{"FailedPrecondition", codes.FailedPrecondition, http.StatusBadRequest},
		{"Aborted", codes.Aborted, http.StatusConflict},
		{"OutOfRange", codes.OutOfRange, http.StatusBadRequest},
		{"Unimplemented", codes.Unimplemented, http.StatusNotImplemented},
		{"Internal", codes.Internal, http.StatusInternalServerError},
		{"Unavailable", codes.Unavailable, http.StatusServiceUnavailable},
		{"DataLoss", codes.DataLoss, http.StatusInternalServerError},
		{"Default", codes.Code(100), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GrpcToHTTPStatus(tt.grpcCode)
			assert.Equal(t, tt.httpCode, result)
		})
	}
}

func TestSanitizeErrorMessage(t *testing.T) {
	tests := []struct {
		name     string
		grpcCode codes.Code
		original string
		expected string
	}{
		{"InvalidArgument", codes.InvalidArgument, "some detailed error", "Invalid request"},
		{"NotFound", codes.NotFound, "user not found", "Resource not found"},
		{"AlreadyExists", codes.AlreadyExists, "user already exists", "Resource already exists"},
		{"PermissionDenied", codes.PermissionDenied, "access denied", "Permission denied"},
		{"Unauthenticated", codes.Unauthenticated, "not logged in", "Authentication required"},
		{"ResourceExhausted", codes.ResourceExhausted, "rate limit exceeded", "Too many requests"},
		{"FailedPrecondition", codes.FailedPrecondition, " precondition failed", "Operation not allowed"},
		{"Unimplemented", codes.Unimplemented, "feature not ready", "Feature not implemented"},
		{"Unavailable", codes.Unavailable, "service down", "Service unavailable"},
		{"Default", codes.Code(100), "some error", "An error occurred"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeErrorMessage(tt.grpcCode, tt.original)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestWriteUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	WriteUnauthorized(w, "test message")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "unauthenticated", resp.Code)
	assert.Equal(t, "test message", resp.Message)
}

func TestWriteForbidden(t *testing.T) {
	w := httptest.NewRecorder()
	WriteForbidden(w)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "permission_denied", resp.Code)
	assert.Equal(t, "Forbidden", resp.Message)
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "bad request")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp ErrorResponse
	err := json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "bad_request", resp.Code)
	assert.Equal(t, "bad request", resp.Message)
}

func TestHandleGrpcError(t *testing.T) {
	tests := []struct {
		name             string
		grpcErr          error
		expectedCode     int
		expectedMsg      string
		expectedGrpcCode string
	}{
		{
			name:             "InvalidArgument",
			grpcErr:          status.Error(codes.InvalidArgument, "invalid input"),
			expectedCode:     http.StatusBadRequest,
			expectedMsg:      "Invalid request",
			expectedGrpcCode: "invalid_argument",
		},
		{
			name:             "NotFound",
			grpcErr:          status.Error(codes.NotFound, "user not found"),
			expectedCode:     http.StatusNotFound,
			expectedMsg:      "Resource not found",
			expectedGrpcCode: "not_found",
		},
		{
			name:             "PermissionDenied",
			grpcErr:          status.Error(codes.PermissionDenied, "access denied"),
			expectedCode:     http.StatusForbidden,
			expectedMsg:      "Permission denied",
			expectedGrpcCode: "permission_denied",
		},
		{
			name:             "Unauthenticated",
			grpcErr:          status.Error(codes.Unauthenticated, "not logged in"),
			expectedCode:     http.StatusUnauthorized,
			expectedMsg:      "Authentication required",
			expectedGrpcCode: "unauthenticated",
		},
		{
			name:             "NonGrpcError",
			grpcErr:          assert.AnError,
			expectedCode:     http.StatusInternalServerError,
			expectedMsg:      "An error occurred",
			expectedGrpcCode: "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			HandleGrpcError(w, tt.grpcErr)

			assert.Equal(t, tt.expectedCode, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var resp ErrorResponse
			err := json.NewDecoder(w.Body).Decode(&resp)
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMsg, resp.Message)
			assert.Equal(t, tt.expectedGrpcCode, resp.Code)
		})
	}
}

func TestHandleGrpcError_PlanLimitDetail(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "plan limit reached")
	st, err := st.WithDetails(&client.PlanLimitDetail{
		Limit:          "active_goals",
		UpgradeTrigger: "goal_limit",
	})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	HandleGrpcError(w, st.Err())

	assert.Equal(t, http.StatusPaymentRequired, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp ErrorResponse
	err = json.NewDecoder(w.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "plan_limit_reached", resp.Code)
	assert.Equal(t, "You have reached the Free plan limit for this feature", resp.Message)
	assert.Equal(t, "active_goals", resp.Limit)
	assert.Equal(t, "goal_limit", resp.UpgradeTrigger)
}

func TestGrpcErrorResponse_ParseErrorsReturn400(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		expectedCode int
		expectedBody string
	}{
		{
			name:         "missing required field",
			err:          fmt.Errorf(`field "fact" is not set`),
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"code":"bad_request","message":"Missing required field"}`,
		},
		{
			name:         "invalid json syntax",
			err:          fmt.Errorf(`invalid character 'x' looking for beginning of value`),
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"code":"bad_request","message":"Invalid field format"}`,
		},
		{
			name:         "unparseable value",
			err:          fmt.Errorf(`cannot parse field page of type int64`),
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"code":"bad_request","message":"Invalid field format"}`,
		},
		{
			name:         "non-grpc non-parse error stays 500",
			err:          fmt.Errorf("connection refused"),
			expectedCode: http.StatusInternalServerError,
			expectedBody: `{"code":"internal_error","message":"An error occurred"}`,
		},
		{
			name:         "grpc invalid argument maps to 400",
			err:          status.Error(codes.InvalidArgument, "fact is required"),
			expectedCode: http.StatusBadRequest,
			expectedBody: `{"code":"invalid_argument","message":"fact is required"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, body := GrpcErrorResponse(tt.err)
			assert.Equal(t, tt.expectedCode, code)

			raw, err := json.Marshal(body)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expectedBody, string(raw))
		})
	}
}
