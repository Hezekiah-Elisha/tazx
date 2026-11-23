package logs

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Hezekiah-Elisha/tazx/internal/config"
	"github.com/Hezekiah-Elisha/tazx/libs"
)

const (
	maxBufferedEntries = 100
	maxScannerToken    = 1024 * 1024
)

type LogEntry struct {
	IP           string    `json:"ip"`
	Timestamp    time.Time `json:"timestamp"`
	HasTimestamp bool      `json:"has_timestamp"`
	Method       string    `json:"method"`
	Route        string    `json:"route"`
	Status       int       `json:"status"`
	Bytes        int64     `json:"bytes"`
	UserAgent    string    `json:"user_agent,omitempty"`
	RawLine      string    `json:"raw_line"`
	Parsed       bool      `json:"parsed"`
	IsError      bool      `json:"is_error"`
}

type RouteTraffic struct {
	Route string `json:"route"`
	Count int    `json:"count"`
}

type LogSummary struct {
	LogPath          string         `json:"log_path"`
	TotalRequests    int            `json:"total_requests"`
	UnparsedLines    int            `json:"unparsed_lines"`
	RequestsPerMin   float64        `json:"requests_per_min"`
	ErrorCount       int            `json:"error_count"`
	HTTPErrorCount   int            `json:"http_error_count"`
	ErrorRate        float64        `json:"error_rate"`
	Status2xx        int            `json:"status_2xx"`
	Status3xx        int            `json:"status_3xx"`
	Status4xx        int            `json:"status_4xx"`
	Status5xx        int            `json:"status_5xx"`
	StatusCounts     map[int]int    `json:"status_counts"`
	RouteCounts      map[string]int `json:"route_counts"`
	IPCounts         map[string]int `json:"ip_counts"`
	SuspiciousRoutes map[string]int `json:"suspicious_routes"`
	FailedLogins     int            `json:"failed_logins"`
	TopRoutes        []RouteTraffic `json:"top_routes"`
	TopIPs           []string       `json:"top_ips"`
	ErrorEntries     []LogEntry     `json:"error_entries,omitempty"`
	RecentEntries    []LogEntry     `json:"recent_entries,omitempty"`
}

var (
	// Standard Nginx / Apache Combined & Common log format regex
	combinedLogRegex = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"(\S+)\s+([^"]+?)(?:\s+HTTP\/[0-9\.]+)?"\s+(\d{3})\s+(\d+|-)(?:\s+"([^"]*)"\s+"([^"]*)")?`)

	unstructuredErrRegex = regexp.MustCompile(`(?i)\b(ERROR|FATAL|CRITICAL|CRIT|ALERT|EMERG)\b`)
	bearerRedactionRegex = regexp.MustCompile(`(?i)(authorization[:=]\s*bearer\s+|bearer\s+)([A-Za-z0-9._\-~+/]+=*)`)
	kvSecretRegex        = regexp.MustCompile(`(?i)\b(token|access_token|refresh_token|api_key|apikey|secret|password|passwd|pwd|auth|jwt|session_id)=([^\s&"']+)`)

	suspiciousKeywords = []string{
		"/wp-admin", "/wp-login", "/.env", "/phpmyadmin", "/.git",
		"/admin", "/xmlrpc.php", "/shell", "/setup.php", "/config.php",
		"/etc/passwd", "/eval", "/vendor", "/composer.json",
	}

	sensitiveQueryParams = []string{
		"token", "access_token", "refresh_token", "auth", "authorization",
		"key", "api_key", "apikey", "secret", "password", "passwd", "pwd",
		"session", "session_id", "sid", "jwt", "bearer", "code",
	}
)

func isSensitiveParam(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	for _, s := range sensitiveQueryParams {
		if lower == s || strings.HasSuffix(lower, "_"+s) {
			return true
		}
	}
	return false
}

// SanitizeRoute strips terminal control characters and redacts sensitive query parameter values.
func SanitizeRoute(route string) string {
	cleaned := libs.SanitizeTerminal(route)
	base, query, hasQuery := strings.Cut(cleaned, "?")
	if !hasQuery {
		return base
	}
	parts := strings.Split(query, "&")
	for i, part := range parts {
		k, _, hasEq := strings.Cut(part, "=")
		if hasEq && isSensitiveParam(k) {
			parts[i] = k + "=[REDACTED]"
		}
	}
	return base + "?" + strings.Join(parts, "&")
}

// SanitizeLogText removes terminal escape sequences and redacts common inline credential patterns.
func SanitizeLogText(line string) string {
	cleaned := libs.SanitizeTerminal(line)
	cleaned = bearerRedactionRegex.ReplaceAllString(cleaned, "${1}[REDACTED]")
	cleaned = kvSecretRegex.ReplaceAllString(cleaned, "${1}=[REDACTED]")
	return cleaned
}

// ParseLine parses a single access or generic log line without inventing missing HTTP fields.
func ParseLine(line string) (*LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	matches := combinedLogRegex.FindStringSubmatch(line)
	if len(matches) >= 7 {
		ip := libs.SanitizeTerminal(matches[1])
		timeStr := matches[2]
		method := libs.SanitizeTerminal(matches[3])
		route := SanitizeRoute(matches[4])
		statusStr := matches[5]
		bytesStr := matches[6]

		status, _ := strconv.Atoi(statusStr)
		var bytes int64
		if bytesStr != "-" {
			bytes, _ = strconv.ParseInt(bytesStr, 10, 64)
		}

		userAgent := ""
		if len(matches) >= 9 {
			userAgent = libs.SanitizeTerminal(matches[8])
		}

		parsedTime, hasTimestamp := parseLogTimestamp(timeStr)

		return &LogEntry{
			IP:           ip,
			Timestamp:    parsedTime,
			HasTimestamp: hasTimestamp,
			Method:       method,
			Route:        route,
			Status:       status,
			Bytes:        bytes,
			UserAgent:    userAgent,
			RawLine:      SanitizeLogText(line),
			Parsed:       true,
			IsError:      status >= 400,
		}, nil
	}

	// Unstructured log line fallback: preserve sanitized line without fabricating HTTP metadata.
	return &LogEntry{
		RawLine: SanitizeLogText(line),
		Parsed:  false,
		IsError: unstructuredErrRegex.MatchString(line),
	}, nil
}

func parseLogTimestamp(raw string) (time.Time, bool) {
	layouts := []string{
		"02/Jan/2006:15:04:05 -0700",
		"02/Jan/2006:15:04:05 MST",
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func appendCapped(slice []LogEntry, entry LogEntry, limit int) []LogEntry {
	if len(slice) < limit {
		return append(slice, entry)
	}
	copy(slice, slice[1:])
	slice[limit-1] = entry
	return slice
}

// AnalyzeLogs reads and summarizes real log data from filePath without creating missing files.
func AnalyzeLogs(ctx context.Context, filePath string) (*LogSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resolved, err := config.ValidateRegularFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to access log file: %w", err)
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}
	defer file.Close()

	summary := &LogSummary{
		LogPath:          filePath,
		StatusCounts:     make(map[int]int),
		RouteCounts:      make(map[string]int),
		IPCounts:         make(map[string]int),
		SuspiciousRoutes: make(map[string]int),
	}

	var minTime, maxTime time.Time
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerToken)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		entry, parseErr := ParseLine(scanner.Text())
		if parseErr != nil {
			continue
		}

		summary.RecentEntries = appendCapped(summary.RecentEntries, *entry, maxBufferedEntries)

		if !entry.Parsed {
			summary.UnparsedLines++
			if entry.IsError {
				summary.ErrorCount++
				summary.ErrorEntries = appendCapped(summary.ErrorEntries, *entry, maxBufferedEntries)
			}
			continue
		}

		summary.TotalRequests++
		summary.StatusCounts[entry.Status]++
		switch {
		case entry.Status >= 200 && entry.Status < 300:
			summary.Status2xx++
		case entry.Status >= 300 && entry.Status < 400:
			summary.Status3xx++
		case entry.Status >= 400 && entry.Status < 500:
			summary.Status4xx++
		case entry.Status >= 500 && entry.Status < 600:
			summary.Status5xx++
		}

		summary.RouteCounts[entry.Route]++
		summary.IPCounts[entry.IP]++

		if entry.HasTimestamp {
			if minTime.IsZero() || entry.Timestamp.Before(minTime) {
				minTime = entry.Timestamp
			}
			if maxTime.IsZero() || entry.Timestamp.After(maxTime) {
				maxTime = entry.Timestamp
			}
		}

		if entry.Status >= 400 {
			summary.ErrorCount++
			summary.HTTPErrorCount++
			summary.ErrorEntries = appendCapped(summary.ErrorEntries, *entry, maxBufferedEntries)
		}

		if entry.Status == 401 || entry.Status == 403 || (strings.Contains(strings.ToLower(entry.Route), "/login") && entry.Status >= 400) {
			summary.FailedLogins++
		}

		lowerRoute := strings.ToLower(entry.Route)
		for _, kw := range suspiciousKeywords {
			if strings.Contains(lowerRoute, kw) {
				summary.SuspiciousRoutes[entry.Route]++
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning log file %s: %w", filePath, err)
	}

	if summary.TotalRequests > 0 {
		summary.ErrorRate = (float64(summary.HTTPErrorCount) / float64(summary.TotalRequests)) * 100
		if !minTime.IsZero() && !maxTime.IsZero() {
			durationMin := maxTime.Sub(minTime).Minutes()
			if durationMin < 1.0 {
				durationMin = 1.0
			}
			summary.RequestsPerMin = float64(summary.TotalRequests) / durationMin
		} else {
			summary.RequestsPerMin = float64(summary.TotalRequests)
		}
	}

	// Top routes sorted deterministically.
	routes := make([]RouteTraffic, 0, len(summary.RouteCounts))
	for r, c := range summary.RouteCounts {
		routes = append(routes, RouteTraffic{Route: r, Count: c})
	}
	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Count == routes[j].Count {
			return routes[i].Route < routes[j].Route
		}
		return routes[i].Count > routes[j].Count
	})
	if len(routes) > 5 {
		summary.TopRoutes = routes[:5]
	} else {
		summary.TopRoutes = routes
	}

	// Top IPs sorted deterministically.
	type ipCount struct {
		ip    string
		count int
	}
	ips := make([]ipCount, 0, len(summary.IPCounts))
	for ip, c := range summary.IPCounts {
		ips = append(ips, ipCount{ip: ip, count: c})
	}
	sort.Slice(ips, func(i, j int) bool {
		if ips[i].count == ips[j].count {
			return ips[i].ip < ips[j].ip
		}
		return ips[i].count > ips[j].count
	})
	for i, item := range ips {
		if i >= 5 {
			break
		}
		summary.TopIPs = append(summary.TopIPs, fmt.Sprintf("%s (%d requests)", item.ip, item.count))
	}

	return summary, nil
}

// TailLogs returns up to the last limit matching entries from filePath using bounded memory.
func TailLogs(ctx context.Context, filePath string, limit int, errorsOnly bool) ([]LogEntry, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	resolved, err := config.ValidateRegularFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to access log file: %w", err)
	}

	file, err := os.Open(resolved)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}
	defer file.Close()

	if limit <= 0 {
		limit = 30
	}

	ring := make([]LogEntry, limit)
	count := 0

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), maxScannerToken)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		entry, parseErr := ParseLine(scanner.Text())
		if parseErr != nil {
			continue
		}
		if errorsOnly && !entry.IsError {
			continue
		}
		ring[count%limit] = *entry
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning log file %s: %w", filePath, err)
	}

	if count == 0 {
		return nil, nil
	}
	if count <= limit {
		return append([]LogEntry(nil), ring[:count]...), nil
	}

	start := count % limit
	entries := make([]LogEntry, 0, limit)
	entries = append(entries, ring[start:]...)
	entries = append(entries, ring[:start]...)
	return entries, nil
}

// FollowLogs tails filePath in real time until ctx is canceled, handling truncation and log rotation.
func FollowLogs(ctx context.Context, filePath string, errorsOnly bool, onEntry func(LogEntry)) error {
	if ctx == nil {
		ctx = context.Background()
	}
	resolved, err := config.ValidateRegularFile(filePath)
	if err != nil {
		return fmt.Errorf("unable to access log file: %w", err)
	}

	file, err := os.Open(resolved)
	if err != nil {
		return fmt.Errorf("unable to open log file: %w", err)
	}
	defer func() {
		if file != nil {
			_ = file.Close()
		}
	}()

	offset, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("failed to seek log file: %w", err)
	}

	openInfo, _ := file.Stat()
	reader := bufio.NewReader(file)
	var partial strings.Builder

	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()

	for {
		for {
			line, readErr := reader.ReadString('\n')
			if line != "" {
				offset += int64(len(line))
				partial.WriteString(line)
				if strings.HasSuffix(line, "\n") {
					fullLine := partial.String()
					partial.Reset()
					if entry, parseErr := ParseLine(fullLine); parseErr == nil {
						if !errorsOnly || entry.IsError {
							onEntry(*entry)
						}
					}
				}
			}
			if readErr != nil {
				if errors.Is(readErr, io.EOF) {
					break
				}
				return readErr
			}
		}

		// Detect file rotation or truncation.
		if pathInfo, statErr := os.Stat(resolved); statErr == nil {
			if openInfo != nil && !os.SameFile(openInfo, pathInfo) {
				reopened, openErr := os.Open(resolved)
				if openErr == nil {
					_ = file.Close()
					file = reopened
					openInfo = pathInfo
					offset = 0
					partial.Reset()
					reader.Reset(file)
				}
			} else if pathInfo.Size() < offset {
				if _, seekErr := file.Seek(0, io.SeekStart); seekErr == nil {
					offset = 0
					partial.Reset()
					reader.Reset(file)
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}
