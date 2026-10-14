package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anzhiyu-c/anheyu-app/pkg/constant"
	"github.com/anzhiyu-c/anheyu-app/pkg/domain/model"
)

func imgbedTestProvider(t *testing.T) IStorageProvider {
	t.Helper()
	p := NewImgBedProvider()
	if p == nil {
		t.Fatal("ImgBed storage provider is not implemented")
	}
	return p
}

func imgbedTestPolicy(endpoint string) *model.StoragePolicy {
	return &model.StoragePolicy{Type: constant.StoragePolicyType("imgbed"), Server: endpoint, SecretKey: "synthetic-token", BasePath: "blog/images", VirtualPath: "/imgbed", MaxSize: 1024, Settings: model.StoragePolicySettings{"upload_method": "server", "upload_channel": "s3", "channel_name": "blog-bucket"}}
}

func TestImgBedFileLifecycle(t *testing.T) {
	const source = "blog/images/immutable-123.png"
	var uploaded []byte
	var deleted, uploadedName bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer synthetic-token" {
			t.Error("missing backend-only token")
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/upload":
			q := r.URL.Query()
			if q.Get("autoRetry") != "false" || q.Get("returnFormat") != "default" || q.Get("uploadChannel") != "s3" || q.Get("channelName") != "blog-bucket" || q.Get("uploadFolder") != "blog/images" {
				t.Errorf("wrong upload options: %v", q)
			}
			if err := r.ParseMultipartForm(1024); err != nil {
				t.Error(err)
				return
			}
			defer r.MultipartForm.RemoveAll()
			file, header, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				return
			}
			defer file.Close()
			uploaded, _ = io.ReadAll(file)
			// A safe unique remote filename must not depend on the blog's display path.
			uploadedName = strings.HasSuffix(header.Filename, ".png") && !strings.Contains(header.Filename, "照片")
			if header.Header.Get("Content-Type") != "image/png" {
				t.Errorf("mime: %s", header.Header.Get("Content-Type"))
			}
			if !strings.Contains(header.Header.Get("Content-Disposition"), `name="file"`) {
				t.Error("multipart names must use browser-compatible quoting for Node/Undici")
			}
			fmt.Fprintf(w, `[{"src":"/file/%s","publicUrl":"https://untrusted.example/file/other"}]`, source)
		case r.URL.Path == "/file/"+source:
			if deleted {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Type", "image/png")
			if r.Method != "HEAD" {
				_, _ = w.Write(uploaded)
			}
		case r.Method == "DELETE" && r.URL.Path == "/api/manage/delete/"+source:
			deleted = true
			_, _ = io.WriteString(w, `{"success":true}`)
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	p := imgbedTestProvider(t)
	policy := imgbedTestPolicy(server.URL)
	payload := []byte("\x89PNG\r\n\x1a\nimage bytes")
	result, err := p.Upload(context.Background(), bytes.NewReader(payload), policy, "/imgbed/旅行/照片.png")
	if err != nil {
		t.Fatal(err)
	}
	if !uploadedName || result.Source != source || result.Size != int64(len(payload)) || result.MimeType != "image/png" || !bytes.Equal(uploaded, payload) {
		t.Fatalf("upload mismatch: %+v", result)
	}
	u, err := p.GetDownloadURL(context.Background(), policy, result.Source, DownloadURLOptions{})
	if err != nil || u != server.URL+"/file/"+source || strings.Contains(u, policy.SecretKey) {
		t.Fatalf("public URL: %q %v", u, err)
	}
	// Rename/move changes the blog's virtual records, preserving the remote ID.
	if err := p.Rename(context.Background(), policy, "/旅行/照片.png", "/其他/新名称.png"); err != nil {
		t.Fatal(err)
	}
	reader, err := p.Get(context.Background(), policy, source)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(reader)
	reader.Close()
	if !bytes.Equal(got, payload) {
		t.Fatal("read bytes changed")
	}
	var stream bytes.Buffer
	if err := p.Stream(context.Background(), policy, source, &stream); err != nil || !bytes.Equal(stream.Bytes(), payload) {
		t.Fatalf("stream: %v", err)
	}
	if exists, err := p.IsExist(context.Background(), policy, source); err != nil || !exists {
		t.Fatalf("exists: %v %v", exists, err)
	}
	if err := p.Delete(context.Background(), policy, []string{source}); err != nil {
		t.Fatal(err)
	}
	if exists, err := p.IsExist(context.Background(), policy, source); err != nil || exists {
		t.Fatalf("deleted file: %v %v", exists, err)
	}
	if err := p.CreateDirectory(context.Background(), policy, "/imgbed/new"); err != nil {
		t.Fatal(err)
	}
	if err := p.DeleteDirectory(context.Background(), policy, "/imgbed/new"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.List(context.Background(), policy, "/imgbed"); !errors.Is(err, ErrFeatureNotSupported) {
		t.Fatalf("remote sync must not import random names: %v", err)
	}
	if _, err := p.GetThumbnail(context.Background(), policy, source, "small"); !errors.Is(err, ErrFeatureNotSupported) {
		t.Fatalf("thumbnail fallback: %v", err)
	}
}

func TestImgBedErrorsNeverExposeCredentials(t *testing.T) {
	for _, status := range []int{401, 403, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				_, _ = io.WriteString(w, "echo synthetic-token")
			}))
			defer server.Close()
			p := imgbedTestProvider(t)
			policy := imgbedTestPolicy(server.URL)
			_, err := p.Upload(context.Background(), strings.NewReader("bytes"), policy, "/imgbed/file.txt")
			if err == nil || strings.Contains(err.Error(), policy.SecretKey) || !strings.Contains(err.Error(), fmt.Sprint(status)) {
				t.Fatalf("unsafe upload error: %v", err)
			}
			if err := p.Delete(context.Background(), policy, []string{"blog/images/file.txt"}); err == nil || strings.Contains(err.Error(), policy.SecretKey) {
				t.Fatalf("unsafe deletion error: %v", err)
			}
		})
	}
}

func TestImgBedRejectsRedirectsAndInvalidUploadResponses(t *testing.T) {
	var leaked bool
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer destination.Close()
	for _, body := range []string{"redirect", `[]`, `[{"src":"https://other.example/file/a"}]`, `[{"src":"/file/../config"}]`, `[{"src":"/file/blog/images/%252e%252e/a"}]`, `[{"src":"/file/blog/images/a?token=synthetic-token"}]`, `[{"src":"/file/outside/a"}]`, `{"success":true}`, `[{"src":"/file/blog/images/a"},{"src":"/file/blog/images/b"}]`} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if body == "redirect" {
					http.Redirect(w, r, destination.URL, 307)
					return
				}
				_, _ = io.WriteString(w, body)
			}))
			defer server.Close()
			p := imgbedTestProvider(t)
			if _, err := p.Upload(context.Background(), strings.NewReader("bytes"), imgbedTestPolicy(server.URL), "/imgbed/file.txt"); err == nil {
				t.Fatal("unsafe response accepted")
			}
		})
	}
	if leaked {
		t.Fatal("authenticated request followed a redirect")
	}
}

func TestImgBedRejectsInvalidPolicyAndSource(t *testing.T) {
	p := imgbedTestProvider(t)
	for _, server := range []string{"", "ftp://example.com", "http://example.com", "https://user:pass@example.com", "https://example.com?key=x", "https://example.com/path", "https://example.com#fragment"} {
		policy := imgbedTestPolicy(server)
		if err := ValidateImgBedPolicy(policy); err == nil {
			t.Errorf("invalid endpoint accepted: %s", server)
		}
	}
	policy := imgbedTestPolicy("https://img.thpvip.xyz")
	if err := ValidateImgBedPolicy(policy); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{"", "../a", "https://other.example/a", "blog/images/%2e%2e/a", "blog/images/a,b", "blog/images/a?key=x", "blog/images/a\\b", "outside/a"} {
		if _, err := p.GetDownloadURL(context.Background(), policy, source, DownloadURLOptions{}); err == nil {
			t.Errorf("invalid source accepted: %q", source)
		}
	}
	policy.SecretKey = ""
	if err := ValidateImgBedPolicy(policy); err == nil {
		t.Fatal("empty token accepted")
	}
	policy.SecretKey = "synthetic-token"
	policy.IsPrivate = true
	if err := ValidateImgBedPolicy(policy); err == nil {
		t.Fatal("private files cannot be exposed by public URLs")
	}
}

func TestImgBedUploadLimitAndCancellation(t *testing.T) {
	p := imgbedTestProvider(t)
	policy := imgbedTestPolicy("http://127.0.0.1:1")
	policy.MaxSize = 3
	if _, err := p.Upload(context.Background(), strings.NewReader("oversize"), policy, "/imgbed/a.txt"); err == nil || !strings.Contains(err.Error(), "大小") {
		t.Fatalf("size limit: %v", err)
	}
	policy.MaxSize = 1024
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Upload(ctx, strings.NewReader("a"), policy, "/imgbed/a.txt"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation must propagate: %v", err)
	}
}

func TestImgBedRejectsFoldersRewrittenByUpstream(t *testing.T) {
	for _, folder := range []string{"blog/my images", "blog/a:b", "blog/(images)", "blog/a+b", "blog/a&b", "blog/a@b", "blog/a'b", "blog/a\"b", "blog/a*", "blog/images ", "blog//images", "blog/./images", "\u00a0", "\u3000"} {
		policy := imgbedTestPolicy("https://img.thpvip.xyz")
		policy.BasePath = folder
		if err := ValidateImgBedPolicy(policy); err == nil {
			t.Errorf("folder rewritten by upstream accepted: %q", folder)
		}
	}
	for _, folder := range []string{"blog/articles", "/blog/images/", "blog/旅行-照片_2026", "blog/images.v2"} {
		policy := imgbedTestPolicy("https://img.thpvip.xyz")
		policy.BasePath = folder
		if err := ValidateImgBedPolicy(policy); err != nil {
			t.Errorf("safe folder rejected: %q: %v", folder, err)
		}
	}
}

func TestImgBedDeleteChecksBusinessResponse(t *testing.T) {
	for _, body := range []string{`{"success":false,"error":"synthetic-token"}`, `{}`, `invalid`} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, body) }))
		p := imgbedTestProvider(t)
		err := p.Delete(context.Background(), imgbedTestPolicy(server.URL), []string{"blog/images/a.txt"})
		server.Close()
		if err == nil || strings.Contains(err.Error(), "synthetic-token") {
			t.Fatalf("business failure: %v", err)
		}
	}
}
