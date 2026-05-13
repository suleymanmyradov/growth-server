package memory

import "context"

// Retriever is the interface for retrieving relevant context from a
// vector store or knowledge base. This is a stub — no implementation
// is provided in pkg/ai. When vector storage is needed (Phase 5+),
// implement this interface in the microservice that owns the vector DB
// and inject it into the agent loop.
//
// Example implementation points:
//   - pgvector via the existing Postgres connection
//   - Pinecone, Weaviate, or Qdrant as external services
//   - In-process FAISS/HNSW index for small corpora
type Retriever interface {
	// Retrieve returns the top-k most relevant documents for the query.
	Retrieve(ctx context.Context, query string, topK int) ([]Document, error)
}

// Document represents a retrieved chunk of text with metadata.
type Document struct {
	Content  string
	Metadata map[string]any
	Score    float64
}
