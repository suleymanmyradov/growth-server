package errors

import (
	"encoding/json"
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
		{"InvalidArgument", codes.InvalidArgument, "some detailed error", "invalid request"},
		{"NotFound", codes.NotFound, "user not found", "resource not found"},
		{"AlreadyExists", codes.AlreadyExists, "user already exists", "resource already exists"},
		{"PermissionDenied", codes.PermissionDenied, "access denied", "permission denied"},
		{"Unauthenticated", codes.Unauthenticated, "not logged in", "authentication required"},
		{"ResourceExhausted", codes.ResourceExhausted, "rate limit exceeded", "too many requests"},
		{"FailedPrecondition", codes.FailedPrecondition, " precondition failed", "operation not allowed"},
		{"Unimplemented", codes.Unimplemented, "feature not ready", "feature not implemented"},
		{"Unavailable", codes.Unavailable, "service down", "service unavailable"},
		{"Default", codes.Code(100), "some error", "an error occurred"},
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
	assert.Equal(t, "forbidden", resp.Message)
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
			expectedMsg:      "invalid request",
			expectedGrpcCode: "invalid_argument",
		},
		{
			name:             "NotFound",
			grpcErr:          status.Error(codes.NotFound, "user not found"),
			expectedCode:     http.StatusNotFound,
			expectedMsg:      "resource not found",
			expectedGrpcCode: "not_found",
		},
		{
			name:             "PermissionDenied",
			grpcErr:          status.Error(codes.PermissionDenied, "access denied"),
			expectedCode:     http.StatusForbidden,
			expectedMsg:      "permission denied",
			expectedGrpcCode: "permission_denied",
		},
		{
			name:             "Unauthenticated",
			grpcErr:          status.Error(codes.Unauthenticated, "not logged in"),
			expectedCode:     http.StatusUnauthorized,
			expectedMsg:      "authentication required",
			expectedGrpcCode: "unauthenticated",
		},
		{
			name:             "NonGrpcError",
			grpcErr:          assert.AnError,
			expectedCode:     http.StatusInternalServerError,
			expectedMsg:      "an error occurred",
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
