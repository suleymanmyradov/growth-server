package logic

import (
	"context"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/filemanager/rpc/pb/filemanager"
)

func testConfig() config.Config {
	var cfg config.Config
	cfg.MinIO.DefaultBucket = "growthmind"
	// A configured region keeps minio-go from doing a network location lookup
	// during presigning, so the test runs without a reachable MinIO.
	cfg.MinIO.Region = "us-east-1"
	return cfg
}

// Presigned URLs must be usable from the browser: signed for the public origin
// (SigV4 is host-bound) and routed through the /files proxy prefix, not the
// cluster-internal MinIO endpoint.
func TestGetPresignedURL_PublicOriginAndPathPrefix(t *testing.T) {
	presignClient, err := minio.New("api.example.com", &minio.Options{
		Creds:  credentials.NewStaticV4("test-access", "test-secret", ""),
		Secure: true,
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatalf("presign client: %v", err)
	}

	svcCtx := &svc.ServiceContext{
		Config:            testConfig(),
		PresignMinio:      presignClient,
		PresignPathPrefix: "/files",
	}

	l := NewGetPresignedURLLogic(context.Background(), svcCtx)
	resp, err := l.GetPresignedURL(&filemanager.GetPresignedURLRequest{
		Key:           "exports/export-user-123.json",
		ExpirySeconds: 900,
	})
	if err != nil {
		t.Fatalf("presign failed: %v", err)
	}

	if !strings.HasPrefix(resp.Url, "https://api.example.com/files/growthmind/exports/") {
		t.Fatalf("presigned url must target the public origin via the proxy prefix, got %s", resp.Url)
	}
	if !strings.Contains(resp.Url, "X-Amz-Signature=") {
		t.Fatalf("presigned url must contain a SigV4 signature, got %s", resp.Url)
	}
}
