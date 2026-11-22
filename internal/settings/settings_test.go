package settings

import (
	"os"
	"strings"
	"testing"
)

func TestAdsTxtManagement(t *testing.T) {
	initial := GetAdsTxt()
	defer func() {
		_ = UpdateAdsTxt(initial)
	}()

	testContent := "google.com, pub-1234567890123456, DIRECT, f08c47fec0942fa0\n"
	err := UpdateAdsTxt(testContent)
	if err != nil {
		t.Fatalf("UpdateAdsTxt failed: %v", err)
	}

	got := GetAdsTxt()
	if got != testContent {
		t.Errorf("GetAdsTxt() = %q, want %q", got, testContent)
	}

	// Verify data/ads.txt file on disk
	diskContent, err := os.ReadFile("data/ads.txt")
	if err != nil {
		t.Fatalf("os.ReadFile(data/ads.txt) failed: %v", err)
	}
	if string(diskContent) != testContent {
		t.Errorf("data/ads.txt on disk = %q, want %q", string(diskContent), testContent)
	}

	// Test delete/clear
	err = UpdateAdsTxt("")
	if err != nil {
		t.Fatalf("UpdateAdsTxt(\"\") failed: %v", err)
	}
	if strings.TrimSpace(GetAdsTxt()) != "" {
		t.Errorf("GetAdsTxt() after delete = %q, want empty", GetAdsTxt())
	}
}
