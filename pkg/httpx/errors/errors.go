package errors

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var snakeCaseRe = regexp.MustCompile("([a-z0-9])([A-Z])")

type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Limit   string `json:"limit,omitempty"`
	UpgradeTrigger string `json:"upgradeTrigger,omitempty"`
}

func WriteUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: "unauthenticated", Message: message})
}

func WriteForbidden(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: "permission_denied", Message: "forbidden"})
}

func WriteError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: httpCodeToSnakeCase(code), Message: message})
}

// WriteParseError sanitizes go-zero parse errors to avoid leaking internal
// struct field names, then writes a uniform 400 BadRequest response.
func WriteParseError(w http.ResponseWriter, err error) {
	sanitized := sanitizeParseError(err.Error())
	WriteError(w, http.StatusBadRequest, sanitized)
}

// sanitizeParseError strips struct field names from go-zero parse errors
// and returns a generic user-friendly message.
func sanitizeParseError(msg string) string {
	// go-zero common patterns:
	// "field XXX is not set"
	// "invalid character ..."
	// "cannot parse ..."
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "is not set"):
		return "missing required field"
	case strings.Contains(lower, "cannot parse") || strings.Contains(lower, "invalid character"):
		return "invalid field format"
	case strings.Contains(lower, "missing field"):
		return "missing required field"
	default:
		return "invalid request"
	}
}

func httpCodeToSnakeCase(code int) string {
	switch code {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusConflict:
		return "conflict"
	case http.StatusInternalServerError:
		return "internal_error"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	case http.StatusNotImplemented:
		return "not_implemented"
	case http.StatusServiceUnavailable:
		return "service_unavailable"
	case http.StatusGatewayTimeout:
		return "gateway_timeout"
	case http.StatusPaymentRequired:
		return "payment_required"
	default:
		return "error"
	}
}

// GrpcToHTTPStatus maps gRPC status codes to HTTP status codes
func GrpcToHTTPStatus(code codes.Code) int {
	switch code {
	case codes.OK:
		return http.StatusOK
	case codes.Canceled:
		return http.StatusRequestTimeout
	case codes.Unknown:
		return http.StatusInternalServerError
	case codes.InvalidArgument:
		return http.StatusBadRequest
	case codes.DeadlineExceeded:
		return http.StatusGatewayTimeout
	case codes.NotFound:
		return http.StatusNotFound
	case codes.AlreadyExists:
		return http.StatusConflict
	case codes.PermissionDenied:
		return http.StatusForbidden
	case codes.Unauthenticated:
		return http.StatusUnauthorized
	case codes.ResourceExhausted:
		return http.StatusTooManyRequests
	case codes.FailedPrecondition:
		return http.StatusBadRequest
	case codes.Aborted:
		return http.StatusConflict
	case codes.OutOfRange:
		return http.StatusBadRequest
	case codes.Unimplemented:
		return http.StatusNotImplemented
	case codes.Internal:
		return http.StatusInternalServerError
	case codes.Unavailable:
		return http.StatusServiceUnavailable
	case codes.DataLoss:
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}

// SanitizeErrorMessage returns a user-friendly error message based on gRPC code
func SanitizeErrorMessage(code codes.Code, _ string) string {
	switch code {
	case codes.InvalidArgument:
		return "invalid request"
	case codes.NotFound:
		return "resource not found"
	case codes.AlreadyExists:
		return "resource already exists"
	case codes.PermissionDenied:
		return "permission denied"
	case codes.Unauthenticated:
		return "authentication required"
	case codes.ResourceExhausted:
		return "too many requests"
	case codes.FailedPrecondition:
		return "operation not allowed"
	case codes.Unimplemented:
		return "feature not implemented"
	case codes.Unavailable:
		return "service unavailable"
	default:
		return "an error occurred"
	}
}

// HandleGrpcError converts gRPC errors to proper HTTP responses with sanitized messages
func HandleGrpcError(w http.ResponseWriter, err error) {
	st, ok := status.FromError(err)
	if !ok {
		// Not a gRPC error, fall back to generic error
		WriteError(w, http.StatusInternalServerError, "an error occurred")
		return
	}

	httpStatus := GrpcToHTTPStatus(st.Code())

	// Check for PlanLimitDetail in gRPC status details.
	for _, d := range st.Details() {
		if detail, ok := d.(interface {
			GetLimit() string
			GetUpgradeTrigger() string
		}); ok {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusPaymentRequired)
			_ = json.NewEncoder(w).Encode(ErrorResponse{
				Code:           "plan_limit_reached",
				Message:        "You have reached the Free plan limit for this feature",
				Limit:          detail.GetLimit(),
				UpgradeTrigger: detail.GetUpgradeTrigger(),
			})
			return
		}
	}

	msg := st.Message()
	sanitizedMessage := SanitizeErrorMessage(st.Code(), msg)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Code:    grpcCodeToSnakeCase(st.Code()),
		Message: sanitizedMessage,
	})
}

func grpcCodeToSnakeCase(code codes.Code) string {
	s := snakeCaseRe.ReplaceAllString(code.String(), "${1}_${2}")
	return strings.ToLower(s)
}
