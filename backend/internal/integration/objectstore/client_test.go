package objectstore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/xizhou0606/aurora-pm/backend/internal/platform/config"
)

func TestNewUsesAProcessUniqueReadinessKeyInArtifactNamespace(t *testing.T) {
	for _, test := range []struct {
		name         string
		globalPrefix string
		wantPrefix   string
	}{
		{name: "global prefix", globalPrefix: "pm", wantPrefix: "pm/workspace/0/.pm-health/object-storage-ready-"},
		{name: "no global prefix", wantPrefix: "workspace/0/.pm-health/object-storage-ready-"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg := config.ObjectStorage{
				Enabled: true, Endpoint: "cos.example.test", Bucket: "pm-files",
				AccessKey: "access", SecretKey: "secret", UseSSL: true, GlobalPrefix: test.globalPrefix,
			}
			first, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			second, err := New(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if first.readinessKeyPrefix == second.readinessKeyPrefix || !strings.HasPrefix(first.readinessKeyPrefix, test.wantPrefix) {
				t.Fatalf("first prefix=%q second prefix=%q", first.readinessKeyPrefix, second.readinessKeyPrefix)
			}
			firstKey := first.nextReadinessKey()
			secondKey := first.nextReadinessKey()
			if firstKey == secondKey || !strings.HasPrefix(firstKey, first.readinessKeyPrefix+"-") {
				t.Fatalf("first key=%q second key=%q", firstKey, secondKey)
			}
		})
	}
}

func TestCheckReadyConcurrentProbesDoNotDeleteEachOther(t *testing.T) {
	const bucket = "pm-files"
	const payload = "pm-object-storage-ready\n"

	var state struct {
		sync.Mutex
		objects    map[string][]byte
		putKeys    []string
		deleteKeys []string
		getCount   int
	}
	state.objects = make(map[string][]byte)
	firstGetStarted := make(chan struct{})
	allowFirstGet := make(chan struct{})
	var firstGetOnce sync.Once
	var firstDeleteOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead && strings.TrimSuffix(request.URL.Path, "/") == "/"+bucket {
			writer.WriteHeader(http.StatusOK)
			return
		}
		key, err := url.PathUnescape(strings.TrimPrefix(request.URL.EscapedPath(), "/"+bucket+"/"))
		if err != nil || key == request.URL.EscapedPath() {
			http.Error(writer, "invalid object key", http.StatusBadRequest)
			return
		}
		// Model a bucket policy that grants object operations only inside the
		// namespace used by real PM artifacts.
		if !strings.HasPrefix(key, "pm/workspace/") {
			writer.Header().Set("Content-Type", "application/xml")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, "<Error><Code>AccessDenied</Code></Error>")
			return
		}
		switch request.Method {
		case http.MethodPut:
			_, readErr := io.Copy(io.Discard, request.Body)
			if readErr != nil {
				http.Error(writer, "read body", http.StatusInternalServerError)
				return
			}
			state.Lock()
			// An S3 server verifies and removes SigV4 streaming framing before storage.
			state.objects[key] = []byte(payload)
			state.putKeys = append(state.putKeys, key)
			state.Unlock()
			writer.Header().Set("ETag", `"probe"`)
			writer.WriteHeader(http.StatusOK)
		case http.MethodGet:
			state.Lock()
			state.getCount++
			isFirstGet := state.getCount == 1
			state.Unlock()
			if isFirstGet {
				firstGetOnce.Do(func() { close(firstGetStarted) })
				select {
				case <-allowFirstGet:
				case <-request.Context().Done():
					return
				}
			}
			state.Lock()
			body, exists := state.objects[key]
			state.Unlock()
			if !exists {
				writer.Header().Set("Content-Type", "application/xml")
				writer.WriteHeader(http.StatusNotFound)
				_, _ = io.WriteString(writer, "<Error><Code>NoSuchKey</Code></Error>")
				return
			}
			writer.Header().Set("Accept-Ranges", "bytes")
			writer.Header().Set("Content-Length", strconv.Itoa(len(body)))
			writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
			writer.Header().Set("ETag", `"probe"`)
			writer.Header().Set("Last-Modified", time.Unix(0, 0).UTC().Format(http.TimeFormat))
			writer.WriteHeader(http.StatusOK)
			_, _ = writer.Write(body)
		case http.MethodDelete:
			state.Lock()
			delete(state.objects, key)
			state.deleteKeys = append(state.deleteKeys, key)
			state.Unlock()
			firstDeleteOnce.Do(func() { close(allowFirstGet) })
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "unsupported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4("access", "secret", ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{
		client: minioClient, bucket: bucket,
		readinessKeyPrefix: "pm/workspace/0/.pm-health/object-storage-ready-test",
	}
	errors := make(chan error, 2)
	go func() { errors <- client.CheckReady(context.Background()) }()
	select {
	case <-firstGetStarted:
	case err := <-errors:
		t.Fatalf("first readiness probe stopped before its read: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("first readiness probe did not reach its read")
	}
	go func() { errors <- client.CheckReady(context.Background()) }()
	for range 2 {
		select {
		case err := <-errors:
			if err != nil {
				t.Errorf("CheckReady() error = %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent readiness probes did not complete")
		}
	}

	state.Lock()
	defer state.Unlock()
	if len(state.putKeys) != 2 || state.putKeys[0] == state.putKeys[1] {
		t.Fatalf("put keys = %v, want two unique keys", state.putKeys)
	}
	slices.Sort(state.putKeys)
	slices.Sort(state.deleteKeys)
	if !slices.Equal(state.deleteKeys, state.putKeys) {
		t.Fatalf("delete keys = %v, want %v", state.deleteKeys, state.putKeys)
	}
	if len(state.objects) != 0 {
		t.Fatalf("objects after cleanup = %v", state.objects)
	}
}

func TestCheckReadyFailsWhenProbeReadIsDenied(t *testing.T) {
	const bucket = "pm-files"

	var state struct {
		sync.Mutex
		putKey    string
		getKey    string
		deleteKey string
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodHead && strings.TrimSuffix(request.URL.Path, "/") == "/"+bucket {
			writer.WriteHeader(http.StatusOK)
			return
		}
		key, err := url.PathUnescape(strings.TrimPrefix(request.URL.EscapedPath(), "/"+bucket+"/"))
		if err != nil || key == request.URL.EscapedPath() {
			http.Error(writer, "invalid object key", http.StatusBadRequest)
			return
		}
		switch request.Method {
		case http.MethodPut:
			_, _ = io.Copy(io.Discard, request.Body)
			state.Lock()
			state.putKey = key
			state.Unlock()
			writer.Header().Set("ETag", `"probe"`)
			writer.WriteHeader(http.StatusOK)
		case http.MethodGet:
			state.Lock()
			state.getKey = key
			state.Unlock()
			writer.Header().Set("Content-Type", "application/xml")
			writer.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(writer, "<Error><Code>AccessDenied</Code></Error>")
		case http.MethodDelete:
			state.Lock()
			state.deleteKey = key
			state.Unlock()
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "unsupported", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	endpoint := strings.TrimPrefix(server.URL, "http://")
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4("access", "secret", ""),
		Secure:       false,
		Region:       "us-east-1",
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	client := &Client{
		client: minioClient, bucket: bucket,
		readinessKeyPrefix: "pm/workspace/0/.pm-health/object-storage-ready-test",
	}

	err = client.CheckReady(context.Background())
	if err == nil {
		t.Fatal("CheckReady() error = nil, want denied probe read to make storage unavailable")
	}
	if !strings.Contains(err.Error(), "read object storage readiness probe") {
		t.Fatalf("CheckReady() error = %v, want probe read context", err)
	}

	state.Lock()
	defer state.Unlock()
	if state.putKey == "" || state.getKey != state.putKey || state.deleteKey != state.putKey {
		t.Fatalf("probe operations put=%q get=%q delete=%q", state.putKey, state.getKey, state.deleteKey)
	}
}

func TestPublicURLUsesConfiguredBaseAndEscapesSegments(t *testing.T) {
	url, ok := PublicURL(config.ObjectStorage{
		PublicBaseURL: "https://cdn.example.test/materials/",
	}, "workspace/1/a file.png")
	if !ok || url != "https://cdn.example.test/materials/workspace/1/a%20file.png" {
		t.Fatalf("url=%q ok=%v", url, ok)
	}
}

func TestPublicURLPreservesSourceVirtualHostedBucketConvention(t *testing.T) {
	url, ok := PublicURL(config.ObjectStorage{
		Endpoint: "https://cos.example.test", Bucket: "pm-files", UseSSL: true,
	}, "pm/bug_materials/a.png")
	if !ok || url != "https://pm-files.cos.example.test/pm/bug_materials/a.png" {
		t.Fatalf("url=%q ok=%v", url, ok)
	}
	url, ok = PublicURL(config.ObjectStorage{
		Endpoint: "pm-files.cos.example.test", Bucket: "pm-files", UseSSL: true,
	}, "a.png")
	if !ok || url != "https://pm-files.cos.example.test/a.png" {
		t.Fatalf("bucket-prefixed url=%q ok=%v", url, ok)
	}
}
