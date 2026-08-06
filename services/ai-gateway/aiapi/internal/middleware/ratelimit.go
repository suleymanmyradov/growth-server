package middleware

import (
	"net/http"
	"strings"

	sharedmw "github.com/suleymanmyradov/growth-server/pkg/httpx/middleware"
)

// RateLimitConfig is re-exported from the shared package so the config struct
// can embed it without changing its import path.
type RateLimitConfig = sharedmw.RateLimitConfig

// RateLimiters is re-exported from the shared package.
type RateLimiters = sharedmw.RateLimiters

// BuildRateLimiters delegates to the shared implementation.
func BuildRateLimiters(cfg RateLimitConfig) *RateLimiters {
	return sharedmw.BuildRateLimiters(cfg)
}

// RateLimitMiddleware applies the ai-gateway's endpoint classification on top
// of the shared rate-limit middleware.
func RateLimitMiddleware(limiters *RateLimiters) func(http.HandlerFunc) http.HandlerFunc {
	return sharedmw.RateLimitMiddleware(limiters, classifyAIGatewayEndpoint)
}

// classifyAIGatewayEndpoint maps a request to its rate-limit bucket. Every
// route served by ai-gateway is an AI endpoint, so most requests get the
// per-user AI bucket. Auth and search buckets are never used here.
func classifyAIGatewayEndpoint(path, method string) sharedmw.RateBucket {
	// Conversational AI: POST /conversations/:id/messages
	if strings.HasPrefix(path, "/api/v1/conversations/") && strings.HasSuffix(path, "/messages") && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	// Weekly review generation
	if path == "/api/v1/weekly-reviews/generate" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	// Personalized coaching (streaming and non-streaming)
	if path == "/api/v1/personalization/coaching" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	if path == "/api/v1/personalization/coaching-stream" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	// Onboarding habits generation
	if path == "/api/v1/personalization/onboarding-habits" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	// Voice turn (multipart → SSE)
	if path == "/api/v1/personalization/voice-turn" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	// Transcribe
	if path == "/api/v1/personalization/transcribe" && method == http.MethodPost {
		return sharedmw.RateBucketAI
	}
	return sharedmw.RateBucketNone
}

// aiExemptPaths are endpoints whose responses intentionally do not use the
// standard data envelope (SSE endpoints and multipart→SSE routes).
var aiExemptPaths = map[string]bool{
	"/api/v1/weekly-reviews/generate-stream":   true,
	"/api/v1/personalization/coaching-stream":  true,
	"/api/v1/personalization/voice-turn":       true,
}

// ResponseShapeMiddleware delegates to the shared implementation with the
// ai-gateway's exempt paths.
func ResponseShapeMiddleware() func(http.HandlerFunc) http.HandlerFunc {
	return sharedmw.ResponseShapeMiddleware(aiExemptPaths)
}
