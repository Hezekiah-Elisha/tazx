package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateVersionTag(t *testing.T) {
	valid := []string{"v1.0.0", "v1.1.0", "v2.0.1-rc1"}
	for _, v := range valid {
		if err := ValidateVersionTag(v); err != nil {
			t.Errorf("ValidateVersionTag(%q) unexpected error: %v", v, err)
		}
	}

	invalid := []string{"", "latest", "1.1.0", "v1.1", "v1.1.0;rm -rf /", "../../v1.0.0"}
	for _, v := range invalid {
		if err := ValidateVersionTag(v); err == nil {
			t.Errorf("ValidateVersionTag(%q) expected error, got nil", v)
		}
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		cur, target string
		want        int
	}{
		{"v1.0.0", "v1.1.0", -1},
		{"v1.1.0", "v1.1.0", 0},
		{"v1.2.0", "v1.1.0", 1},
		{"v1.1.0-rc1", "v1.1.0", -1},
		{"v1.1.0", "v1.1.0-rc1", 1},
		{"v1.10.0", "v1.2.0", 1},
	}
	for _, tc := range tests {
		if got := CompareVersions(tc.cur, tc.target); got != tc.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.cur, tc.target, got, tc.want)
		}
	}
}

func TestParseChecksum(t *testing.T) {
	sum := strings.Repeat("a", 64)
	content := fmt.Sprintf("%s  tazx-linux-amd64\n%s *tazx-darwin-arm64\n", sum, strings.Repeat("b", 64))

	got, err := ParseChecksum(content, "tazx-linux-amd64")
	if err != nil || got != sum {
		t.Fatalf("ParseChecksum linux = %q / %v", got, err)
	}
	gotMac, err := ParseChecksum(content, "tazx-darwin-arm64")
	if err != nil || gotMac != strings.Repeat("b", 64) {
		t.Fatalf("ParseChecksum darwin = %q / %v", gotMac, err)
	}
	if _, err := ParseChecksum(content, "tazx-windows-amd64"); err == nil {
		t.Error("expected missing asset error")
	}
}

func TestAssetName(t *testing.T) {
	if got, err := AssetName("linux", "amd64"); err != nil || got != "tazx-linux-amd64" {
		t.Errorf("unexpected AssetName: %q / %v", got, err)
	}
	if _, err := AssetName("windows", "amd64"); err == nil {
		t.Error("expected unsupported OS error")
	}
}

func TestCheckAndUpdateWorkflow(t *testing.T) {
	newPayload := []byte("#!/bin/sh\necho v1.2.0\n")
	h := sha256.Sum256(newPayload)
	newSum := hex.EncodeToString(h[:])

	var srv *httptest.Server
	serveBadSum := false

	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/Hezekiah-Elisha/tazx/releases/latest":
			_, _ = w.Write([]byte(`{"tag_name":"v1.2.0","html_url":"https://example.com/releases/v1.2.0"}`))
		case "/Hezekiah-Elisha/tazx/releases/download/v1.2.0/checksums.txt":
			sum := newSum
			if serveBadSum {
				sum = strings.Repeat("0", 64)
			}
			_, _ = fmt.Fprintf(w, "%s  tazx-linux-amd64\n", sum)
		case "/Hezekiah-Elisha/tazx/releases/download/v1.2.0/tazx-linux-amd64":
			_, _ = w.Write(newPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// Default client must reject insecure HTTP.
	strictClient := NewClient()
	strictClient.APIBaseURL = srv.URL
	if _, _, err := strictClient.FetchLatestVersion(context.Background()); err == nil {
		t.Fatal("expected strict HTTPS client to reject http:// URL")
	}

	client := NewClient()
	client.APIBaseURL = srv.URL
	client.DownloadBaseURL = srv.URL
	client.GOOS = "linux"
	client.GOARCH = "amd64"
	client.AllowInsecureHTTP = true

	dir := t.TempDir()
	exePath := filepath.Join(dir, "tazx")
	origPayload := []byte("old-binary-v1.0.0")
	if err := os.WriteFile(exePath, origPayload, 0755); err != nil {
		t.Fatal(err)
	}

	// 1. CheckForUpdate reports update available.
	chk, err := client.CheckForUpdate(context.Background(), "v1.1.0", "", exePath)
	if err != nil {
		t.Fatalf("CheckForUpdate failed: %v", err)
	}
	if !chk.UpdateAvailable || chk.LatestVersion != "v1.2.0" {
		t.Errorf("unexpected CheckResult: %+v", chk)
	}

	// 2. Checksum mismatch must fail and leave original binary untouched.
	serveBadSum = true
	if _, err := client.Update(context.Background(), "v1.1.0", exePath, Options{}); err == nil {
		t.Fatal("expected checksum mismatch error")
	}
	afterFail, _ := os.ReadFile(exePath)
	if string(afterFail) != string(origPayload) {
		t.Fatalf("original binary was modified on checksum failure")
	}

	// 3. Valid checksum updates binary atomically with 0755 permissions.
	serveBadSum = false
	res, err := client.Update(context.Background(), "v1.1.0", exePath, Options{})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if !res.Updated || res.TargetVersion != "v1.2.0" || res.Checksum != newSum {
		t.Errorf("unexpected UpdateResult: %+v", res)
	}
	updatedBytes, err := os.ReadFile(exePath)
	if err != nil || string(updatedBytes) != string(newPayload) {
		t.Fatalf("binary content not updated properly")
	}
	info, err := os.Stat(exePath)
	if err != nil || info.Mode().Perm() != 0755 {
		t.Errorf("expected 0755 permissions, got %v / %v", info.Mode().Perm(), err)
	}

	// 4. Already up to date skips download unless Force is true.
	resNoop, err := client.Update(context.Background(), "v1.2.0", exePath, Options{})
	if err != nil || resNoop.Updated {
		t.Errorf("expected no-op when already on v1.2.0, got %+v / %v", resNoop, err)
	}
	resForce, err := client.Update(context.Background(), "v1.2.0", exePath, Options{Force: true})
	if err != nil || !resForce.Updated {
		t.Errorf("expected Force update to reinstall, got %+v / %v", resForce, err)
	}
}
