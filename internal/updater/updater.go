package updater

import (
	"bufio"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRepo         = "Hezekiah-Elisha/tazx"
	DefaultAPIBase      = "https://api.github.com"
	DefaultDownloadBase = "https://github.com"

	maxBinaryBytes   int64 = 100 * 1024 * 1024
	maxChecksumBytes int64 = 1 * 1024 * 1024
)

var validTagRegex = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+([-.][a-zA-Z0-9.]+)?$`)

// CheckResult describes whether an update is available.
type CheckResult struct {
	CurrentVersion  string `json:"current_version"`
	LatestVersion   string `json:"latest_version"`
	UpdateAvailable bool   `json:"update_available"`
	ReleaseURL      string `json:"release_url,omitempty"`
	ExecutablePath  string `json:"executable_path"`
	AssetName       string `json:"asset_name"`
}

// UpdateResult describes the outcome of an update operation.
type UpdateResult struct {
	CurrentVersion string `json:"current_version"`
	TargetVersion  string `json:"target_version"`
	Updated        bool   `json:"updated"`
	ExecutablePath string `json:"executable_path"`
	Checksum       string `json:"checksum,omitempty"`
	Message        string `json:"message"`
}

// Options controls update behavior.
type Options struct {
	TargetVersion string
	Force         bool
}

// Client checks and applies signed-checksum HTTPS binary updates.
type Client struct {
	Repo              string
	APIBaseURL        string
	DownloadBaseURL   string
	HTTPClient        *http.Client
	GOOS              string
	GOARCH            string
	AllowInsecureHTTP bool
}

// NewClient returns an update Client configured with TLS 1.2+ and HTTPS redirect enforcement.
func NewClient() *Client {
	c := &Client{
		Repo:            DefaultRepo,
		APIBaseURL:      DefaultAPIBase,
		DownloadBaseURL: DefaultDownloadBase,
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
	}
	c.HTTPClient = &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return errors.New("too many redirects")
			}
			if !c.AllowInsecureHTTP && !strings.EqualFold(req.URL.Scheme, "https") {
				return fmt.Errorf("refusing insecure redirect to %s", req.URL.Scheme)
			}
			return nil
		},
	}
	return c
}

// ValidateVersionTag checks that tag is a clean semantic version tag such as v1.1.0.
func ValidateVersionTag(tag string) error {
	trimmed := strings.TrimSpace(tag)
	if !validTagRegex.MatchString(trimmed) {
		return fmt.Errorf("invalid version tag %q (expected format like v1.1.0)", tag)
	}
	return nil
}

type semver struct {
	major int
	minor int
	patch int
	pre   string
	valid bool
}

func parseSemver(raw string) semver {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "v"), "V")
	if s == "" {
		return semver{}
	}
	core, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(core, ".")
	if len(parts) != 3 {
		return semver{}
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	pat, err3 := strconv.Atoi(parts[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return semver{}
	}
	return semver{major: maj, minor: min, patch: pat, pre: pre, valid: true}
}

// CompareVersions compares two semantic versions, returning -1 if current < target,
// 0 if equal, and 1 if current > target.
func CompareVersions(current, target string) int {
	a := parseSemver(current)
	b := parseSemver(target)
	if !a.valid || !b.valid {
		ca := strings.TrimPrefix(strings.TrimSpace(current), "v")
		cb := strings.TrimPrefix(strings.TrimSpace(target), "v")
		if ca < cb {
			return -1
		}
		if ca > cb {
			return 1
		}
		return 0
	}

	if a.major != b.major {
		if a.major < b.major {
			return -1
		}
		return 1
	}
	if a.minor != b.minor {
		if a.minor < b.minor {
			return -1
		}
		return 1
	}
	if a.patch != b.patch {
		if a.patch < b.patch {
			return -1
		}
		return 1
	}

	if a.pre == b.pre {
		return 0
	}
	// Stable release (empty pre-release) sorts higher than pre-release.
	if a.pre == "" {
		return 1
	}
	if b.pre == "" {
		return -1
	}
	if a.pre < b.pre {
		return -1
	}
	return 1
}

// AssetName returns the expected release asset filename for goos/goarch.
func AssetName(goos, goarch string) (string, error) {
	switch goos {
	case "linux", "darwin":
	default:
		return "", fmt.Errorf("unsupported operating system %q for binary updates (supported: linux, darwin)", goos)
	}
	switch goarch {
	case "amd64", "arm64":
	default:
		return "", fmt.Errorf("unsupported architecture %q for binary updates (supported: amd64, arm64)", goarch)
	}
	return fmt.Sprintf("tazx-%s-%s", goos, goarch), nil
}

// ParseChecksum finds the SHA-256 hex digest for assetName inside checksumsContent.
func ParseChecksum(checksumsContent, assetName string) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(checksumsContent))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		digest := strings.ToLower(fields[0])
		fileField := strings.TrimPrefix(fields[1], "*")
		fileField = strings.TrimPrefix(fileField, "./")
		if fileField != assetName {
			continue
		}
		if len(digest) != 64 {
			return "", fmt.Errorf("invalid SHA-256 digest length for %s", assetName)
		}
		if _, err := hex.DecodeString(digest); err != nil {
			return "", fmt.Errorf("invalid SHA-256 hex digest for %s: %w", assetName, err)
		}
		return digest, nil
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("checksum for %s not found in checksums.txt", assetName)
}

// VerifyFileSHA256 hashes path and confirms it matches expectedHex.
func VerifyFileSHA256(path, expectedHex string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expectedHex) {
		return actual, fmt.Errorf("SHA-256 mismatch: expected %s, got %s", strings.ToLower(expectedHex), actual)
	}
	return actual, nil
}

func resolveExecutable(exePath string) (string, error) {
	if exePath != "" {
		return filepath.Clean(exePath), nil
	}
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("unable to determine current executable path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err == nil {
		return resolved, nil
	}
	return exe, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return NewClient().HTTPClient
}

func (c *Client) doGet(ctx context.Context, rawURL string) (*http.Response, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", rawURL, err)
	}
	if !c.AllowInsecureHTTP && !strings.EqualFold(parsed.Scheme, "https") {
		return nil, fmt.Errorf("refusing non-HTTPS update URL %q", rawURL)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "tazx-updater")
	return c.httpClient().Do(req)
}

type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// FetchLatestVersion queries GitHub Releases for the latest published tag.
func (c *Client) FetchLatestVersion(ctx context.Context) (string, string, error) {
	base := strings.TrimRight(c.APIBaseURL, "/")
	if base == "" {
		base = DefaultAPIBase
	}
	repo := c.Repo
	if repo == "" {
		repo = DefaultRepo
	}

	endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", base, repo)
	resp, err := c.doGet(ctx, endpoint)
	if err != nil {
		return "", "", fmt.Errorf("failed to query latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", "", fmt.Errorf("no published GitHub release found for %s (if building locally, run 'bash scripts/install.sh --from-source')", repo)
	}
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("release API returned HTTP %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxChecksumBytes)).Decode(&rel); err != nil {
		return "", "", fmt.Errorf("failed to parse release metadata: %w", err)
	}
	if err := ValidateVersionTag(rel.TagName); err != nil {
		return "", "", err
	}
	return rel.TagName, rel.HTMLURL, nil
}

// CheckForUpdate inspects whether targetVersion (or latest release) is newer than currentVersion.
func (c *Client) CheckForUpdate(ctx context.Context, currentVersion, targetVersion, exePath string) (*CheckResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	resolvedExe, err := resolveExecutable(exePath)
	if err != nil {
		return nil, err
	}
	asset, err := AssetName(c.GOOS, c.GOARCH)
	if err != nil {
		return nil, err
	}

	var latestTag, relURL string
	if strings.TrimSpace(targetVersion) != "" {
		if err := ValidateVersionTag(targetVersion); err != nil {
			return nil, err
		}
		latestTag = strings.TrimSpace(targetVersion)
	} else {
		latestTag, relURL, err = c.FetchLatestVersion(ctx)
		if err != nil {
			return nil, err
		}
	}

	return &CheckResult{
		CurrentVersion:  currentVersion,
		LatestVersion:   latestTag,
		UpdateAvailable: CompareVersions(currentVersion, latestTag) < 0,
		ReleaseURL:      relURL,
		ExecutablePath:  resolvedExe,
		AssetName:       asset,
	}, nil
}

// Update downloads, SHA-256 verifies, and atomically replaces exePath with the target release binary.
func (c *Client) Update(ctx context.Context, currentVersion, exePath string, opts Options) (*UpdateResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	resolvedExe, err := resolveExecutable(exePath)
	if err != nil {
		return nil, err
	}
	asset, err := AssetName(c.GOOS, c.GOARCH)
	if err != nil {
		return nil, err
	}

	var targetTag string
	if strings.TrimSpace(opts.TargetVersion) != "" {
		if err := ValidateVersionTag(opts.TargetVersion); err != nil {
			return nil, err
		}
		targetTag = strings.TrimSpace(opts.TargetVersion)
	} else {
		targetTag, _, err = c.FetchLatestVersion(ctx)
		if err != nil {
			return nil, err
		}
	}

	if !opts.Force && CompareVersions(currentVersion, targetTag) >= 0 {
		return &UpdateResult{
			CurrentVersion: currentVersion,
			TargetVersion:  targetTag,
			Updated:        false,
			ExecutablePath: resolvedExe,
			Message:        fmt.Sprintf("Tazx is already up to date (%s).", currentVersion),
		}, nil
	}

	targetDir := filepath.Dir(resolvedExe)
	tmpFile, err := os.CreateTemp(targetDir, ".tazx-update-*")
	if err != nil {
		return nil, fmt.Errorf("cannot write update to %s (%w); if installed system-wide, run with sudo or update via 'bash scripts/install.sh'", targetDir, err)
	}
	tmpPath := tmpFile.Name()
	renamed := false
	defer func() {
		_ = tmpFile.Close()
		if !renamed {
			_ = os.Remove(tmpPath)
		}
	}()

	dlBase := strings.TrimRight(c.DownloadBaseURL, "/")
	if dlBase == "" {
		dlBase = DefaultDownloadBase
	}
	repo := c.Repo
	if repo == "" {
		repo = DefaultRepo
	}

	checksumURL := fmt.Sprintf("%s/%s/releases/download/%s/checksums.txt", dlBase, repo, targetTag)
	chkResp, err := c.doGet(ctx, checksumURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download checksums.txt: %w", err)
	}
	defer chkResp.Body.Close()
	if chkResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download checksums.txt: HTTP %d", chkResp.StatusCode)
	}
	chkBytes, err := io.ReadAll(io.LimitReader(chkResp.Body, maxChecksumBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to read checksums.txt: %w", err)
	}
	expectedSHA, err := ParseChecksum(string(chkBytes), asset)
	if err != nil {
		return nil, err
	}

	assetURL := fmt.Sprintf("%s/%s/releases/download/%s/%s", dlBase, repo, targetTag, asset)
	binResp, err := c.doGet(ctx, assetURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download %s: %w", asset, err)
	}
	defer binResp.Body.Close()
	if binResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download %s: HTTP %d", asset, binResp.StatusCode)
	}

	written, err := io.Copy(tmpFile, io.LimitReader(binResp.Body, maxBinaryBytes+1))
	if err != nil {
		return nil, fmt.Errorf("failed to write downloaded binary: %w", err)
	}
	if written > maxBinaryBytes {
		return nil, fmt.Errorf("downloaded binary exceeds maximum size limit (%d bytes)", maxBinaryBytes)
	}
	if written == 0 {
		return nil, errors.New("downloaded binary is empty")
	}
	if err := tmpFile.Chmod(0755); err != nil {
		return nil, fmt.Errorf("failed to set executable permissions: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return nil, fmt.Errorf("failed to flush binary to disk: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temporary binary: %w", err)
	}

	verifiedSHA, err := VerifyFileSHA256(tmpPath, expectedSHA)
	if err != nil {
		return nil, err
	}

	if err := os.Rename(tmpPath, resolvedExe); err != nil {
		return nil, fmt.Errorf("failed to replace binary at %s: %w", resolvedExe, err)
	}
	renamed = true

	return &UpdateResult{
		CurrentVersion: currentVersion,
		TargetVersion:  targetTag,
		Updated:        true,
		ExecutablePath: resolvedExe,
		Checksum:       verifiedSHA,
		Message:        fmt.Sprintf("Updated Tazx from %s to %s.", currentVersion, targetTag),
	}, nil
}
