package notifications

import (
	"testing"

	"github.com/google/uuid"
)

func TestDeterministicChunkEventID_Stable(t *testing.T) {
	broadcastID := uuid.NewString()
	id1 := deterministicChunkEventID(broadcastID, 1)
	id2 := deterministicChunkEventID(broadcastID, 1)
	if id1 != id2 {
		t.Errorf("expected deterministic ID to be stable: %s != %s", id1, id2)
	}
	// Must be a valid UUID.
	if _, err := uuid.Parse(id1); err != nil {
		t.Errorf("expected valid UUID, got %s: %v", id1, err)
	}
}

func TestDeterministicChunkEventID_DifferentChunks(t *testing.T) {
	broadcastID := uuid.NewString()
	id1 := deterministicChunkEventID(broadcastID, 1)
	id2 := deterministicChunkEventID(broadcastID, 2)
	if id1 == id2 {
		t.Error("expected different event IDs for different chunk indices")
	}
}

func TestDeterministicChunkEventID_DifferentBroadcasts(t *testing.T) {
	b1 := uuid.NewString()
	b2 := uuid.NewString()
	id1 := deterministicChunkEventID(b1, 1)
	id2 := deterministicChunkEventID(b2, 1)
	if id1 == id2 {
		t.Error("expected different event IDs for different broadcast IDs")
	}
}

func TestChunkIDs(t *testing.T) {
	ids := make([]string, 5000)
	for i := range ids {
		ids[i] = "user-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}

	chunks := chunkIDs(ids, 2000)
	if len(chunks) != 3 {
		t.Errorf("expected 3 chunks for 5000 ids with size 2000, got %d", len(chunks))
	}
	if len(chunks[0]) != 2000 || len(chunks[1]) != 2000 || len(chunks[2]) != 1000 {
		t.Errorf("expected chunk sizes 2000/2000/1000, got %d/%d/%d",
			len(chunks[0]), len(chunks[1]), len(chunks[2]))
	}

	// Verify all IDs are present.
	seen := make(map[string]bool)
	for _, chunk := range chunks {
		for _, id := range chunk {
			seen[id] = true
		}
	}
	if len(seen) != 5000 {
		t.Errorf("expected 5000 unique ids across chunks, got %d", len(seen))
	}
}

func TestChunkIDs_SingleChunk(t *testing.T) {
	ids := []string{"a", "b", "c"}
	chunks := chunkIDs(ids, 100)
	if len(chunks) != 1 || len(chunks[0]) != 3 {
		t.Errorf("expected single chunk of 3, got %d chunks", len(chunks))
	}
}

func TestChunkIDs_Empty(t *testing.T) {
	chunks := chunkIDs(nil, 100)
	if chunks != nil {
		t.Errorf("expected nil for empty input, got %v", chunks)
	}
}
