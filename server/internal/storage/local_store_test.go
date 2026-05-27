package storage

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalStorePutObjectWritesFileAndReturnsPublicURL(t *testing.T) {
	store := NewLocalStore(LocalStoreOptions{
		RootDir:       t.TempDir(),
		PublicBaseURL: "/uploads",
	})

	result, err := store.PutObject(context.Background(), PutObjectInput{
		Key:         "tenant-logos/20260527/logo.png",
		FileName:    "logo.png",
		ContentType: "image/png",
		Size:        4,
		Body:        strings.NewReader("logo"),
	})
	if err != nil {
		t.Fatalf("put object: %v", err)
	}

	if result.URL != "/uploads/tenant-logos/20260527/logo.png" || result.Size != 4 {
		t.Fatalf("unexpected result: %#v", result)
	}
	content, err := os.ReadFile(filepath.Join(store.rootDir, "tenant-logos", "20260527", "logo.png"))
	if err != nil {
		t.Fatalf("read stored file: %v", err)
	}
	if string(content) != "logo" {
		t.Fatalf("stored content = %q", string(content))
	}
}

func TestLocalStoreRejectsUnsafeObjectKey(t *testing.T) {
	store := NewLocalStore(LocalStoreOptions{RootDir: t.TempDir()})

	_, err := store.PutObject(context.Background(), PutObjectInput{
		Key:  "../logo.png",
		Body: strings.NewReader("logo"),
	})
	if !errors.Is(err, ErrInvalidObjectKey) {
		t.Fatalf("expected ErrInvalidObjectKey, got %v", err)
	}
}
