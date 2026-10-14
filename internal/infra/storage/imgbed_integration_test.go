package storage

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"net/http"
	"os"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

// Run only against the isolated fixture created by tools/testing/imgbed-integration.mjs.
func TestImgBedLocalServerIntegration(t *testing.T) {
	endpoint := os.Getenv("IMGBED_INTEGRATION_URL")
	token := os.Getenv("IMGBED_INTEGRATION_TOKEN")
	if endpoint == "" || token == "" {
		t.Skip("isolated ImgBed fixture not running")
	}
	policy := &model.StoragePolicy{Type: constant.PolicyTypeImgBed, Server: endpoint, SecretKey: token, BasePath: "blog/integration", VirtualPath: "/imgbed", MaxSize: 1024, Settings: model.StoragePolicySettings{"upload_method": "server", "upload_channel": "cfr2"}}
	p := NewImgBedProvider()
	ctx := context.Background()
	png, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=")
	result, err := p.Upload(ctx, bytes.NewReader(png), policy, "/imgbed/旅行/照片.png")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Delete(ctx, policy, []string{result.Source}) })
	reader, err := p.Get(ctx, policy, result.Source)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(got, png) {
		t.Fatalf("uploaded bytes differ: %v", err)
	}
	publicURL, err := p.GetDownloadURL(ctx, policy, result.Source, DownloadURLOptions{})
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(publicURL)
	if err != nil {
		t.Fatal(err)
	}
	publicBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || !bytes.Equal(publicBytes, png) {
		t.Fatalf("public read: HTTP %d", resp.StatusCode)
	}
	if err := p.Rename(ctx, policy, "/旅行/照片.png", "/其他/新名称.png"); err != nil {
		t.Fatal(err)
	}
	if exists, err := p.IsExist(ctx, policy, result.Source); err != nil || !exists {
		t.Fatalf("after rename: %v %v", exists, err)
	}
	wrong := *policy
	wrong.SecretKey = "deliberately-invalid-test-token"
	if _, err := p.Upload(ctx, bytes.NewReader(png), &wrong, "/imgbed/bad.png"); err == nil {
		t.Fatal("invalid token upload accepted")
	}
	if err := p.Delete(ctx, policy, []string{result.Source}); err != nil {
		t.Fatal(err)
	}
	if exists, err := p.IsExist(ctx, policy, result.Source); err != nil || exists {
		t.Fatalf("after delete: %v %v", exists, err)
	}
	t.Log("PASS: real ImgBed multipart upload, exact bytes, public read, stable rename, invalid token rejection and deletion")
}
