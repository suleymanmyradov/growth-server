package logic

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Real magic bytes for the formats clients are allowed to upload.
var (
	pngBytes  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	jpegBytes = []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	gifBytes  = []byte("GIF89a....")
	htmlBytes = []byte("<!DOCTYPE html><html><body><script>alert(1)</script></body></html>")
	svgBytes  = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
	jsonBytes = []byte(`{"habits":[],"goals":[]}`)
)

func TestValidateUpload_Allowed(t *testing.T) {
	tests := []struct {
		name   string
		folder string
		data   []byte
	}{
		{"png avatar", "avatars", pngBytes},
		{"jpeg avatar", "avatars", jpegBytes},
		{"gif article cover", "articles", gifBytes},
		{"json export", "exports", jsonBytes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, validateUpload(tt.folder, tt.data))
		})
	}
}

func TestValidateUpload_Rejected(t *testing.T) {
	tests := []struct {
		name   string
		folder string
		data   []byte
	}{
		{"html disguised as png", "avatars", htmlBytes},
		{"svg in articles", "articles", svgBytes},
		{"html in exports", "exports", htmlBytes},
		{"json in avatars", "avatars", jsonBytes},
		{"unknown folder", "unknown", pngBytes},
		{"path traversal folder", "../avatars", pngBytes},
		{"empty payload", "avatars", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, validateUpload(tt.folder, tt.data))
		})
	}
}
