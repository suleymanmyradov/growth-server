package articles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	testPngBytes  = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R'}
	testJpegBytes = []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F'}
	testGifBytes  = []byte("GIF89a....")
	testWebpBytes = []byte("RIFF\x00\x00\x00\x00WEBPVP8 ")
	testHtmlBytes = []byte("<!DOCTYPE html><html><body><script>alert(1)</script></body></html>")
	testSvgBytes  = []byte(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)
)

func TestDetectImageType_Allowed(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		wantTyp string
	}{
		{"png", testPngBytes, "image/png"},
		{"jpeg", testJpegBytes, "image/jpeg"},
		{"gif", testGifBytes, "image/gif"},
		{"webp", testWebpBytes, "image/webp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			typ, err := detectImageType(tt.data)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantTyp, typ)
		})
	}
}

func TestDetectImageType_Rejected(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{"html disguised as image", testHtmlBytes},
		{"svg with script", testSvgBytes},
		{"empty payload", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := detectImageType(tt.data)
			assert.Error(t, err)
			assert.Equal(t, codes.InvalidArgument, status.Code(err))
		})
	}
}
