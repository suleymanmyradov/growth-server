package mdpropagate

// Metadata keys for gRPC context propagation.
// All keys use the "growth-" prefix to avoid collisions with other middleware.
const (
	MDAuthorization = "growth-authorization"
)
