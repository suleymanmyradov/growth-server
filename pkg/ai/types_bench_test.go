package ai

import "testing"

func makeMessages(n int) []Message {
	msgs := make([]Message, n)
	for i := range msgs {
		msgs[i] = Message{Role: RoleUser, Content: "Hello world message content for benchmarking"}
	}
	return msgs
}

func BenchmarkToEinoMessages(b *testing.B) {
	msgs := makeMessages(20)
	b.ReportAllocs()

	for b.Loop() {
		_ = toEinoMessages(msgs, "You are a helpful assistant")
	}
}
