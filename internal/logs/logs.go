package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type LogEntry struct {
	IP        string
	Timestamp time.Time
	Method    string
	Route     string
	Status    int
	Bytes     int64
	UserAgent string
	RawLine   string
}

type RouteTraffic struct {
	Route string
	Count int
}

type LogSummary struct {
	TotalRequests     int
	RequestsPerMin    float64
	ErrorCount        int
	StatusCounts      map[int]int
	RouteCounts       map[string]int
	IPCounts          map[string]int
	SuspiciousRoutes  map[string]int
	FailedLogins      int
	TopRoutes         []RouteTraffic
	TopIPs            []string
	ErrorEntries      []LogEntry
	RecentEntries     []LogEntry
}

var (
	// Standard Nginx / Apache combined log format regex
	// 127.0.0.1 - - [02/Aug/2026:14:00:00 +0300] "GET /api/v1/users HTTP/1.1" 200 1234 "-" "Mozilla/5.0"
	combinedLogRegex = regexp.MustCompile(`^(\S+)\s+\S+\s+\S+\s+\[([^\]]+)\]\s+"(\S+)\s+([^"]+?)(?:\s+HTTP\/[0-9\.]+)?"\s+(\d{3})\s+(\d+|-)(?:\s+"([^"]*)"\s+"([^"]*)")?`)

	suspiciousKeywords = []string{
		"/wp-admin", "/wp-login", "/.env", "/phpmyadmin", "/.git",
		"/admin", "/xmlrpc.php", "/shell", "/setup.php", "/config.php",
		"/etc/passwd", "/eval", "/vendor", "/composer.json",
	}
)

func ParseLine(line string) (*LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, fmt.Errorf("empty line")
	}

	matches := combinedLogRegex.FindStringSubmatch(line)
	if len(matches) >= 6 {
		ip := matches[1]
		timeStr := matches[2]
		method := matches[3]
		route := matches[4]
		statusStr := matches[5]
		bytesStr := matches[6]

		status, _ := strconv.Atoi(statusStr)
		var bytes int64
		if bytesStr != "-" {
			bytes, _ = strconv.ParseInt(bytesStr, 10, 64)
		}

		userAgent := ""
		if len(matches) >= 9 {
			userAgent = matches[8]
		}

		parsedTime, err := time.Parse("02/Jan/2006:15:04:05 -0700", timeStr)
		if err != nil {
			parsedTime = time.Now()
		}

		return &LogEntry{
			IP:        ip,
			Timestamp: parsedTime,
			Method:    method,
			Route:     route,
			Status:    status,
			Bytes:     bytes,
			UserAgent: userAgent,
			RawLine:   line,
		}, nil
	}

	// Fallback simple line parser if not combined log format
	parts := strings.Fields(line)
	entry := &LogEntry{RawLine: line, Timestamp: time.Now()}
	for _, part := range parts {
		if strings.HasPrefix(part, "/") {
			entry.Route = part
			break
		}
	}
	if entry.Route == "" {
		entry.Route = "/"
	}
	if strings.Contains(strings.ToUpper(line), "ERROR") || strings.Contains(line, "500") || strings.Contains(line, "404") {
		entry.Status = 500
	} else {
		entry.Status = 200
	}
	return entry, nil
}

func EnsureSampleLog(filePath string) error {
	if _, err := os.Stat(filePath); err == nil {
		return nil // File already exists
	}

	dir := filepath.Dir(filePath)
	if dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	now := time.Now()
	nowStr := now.Format("02/Jan/2006:15:04:05 -0700")

	sampleLogs := []string{
		fmt.Sprintf(`192.168.1.10 - - [%s] "GET / HTTP/1.1" 200 4523 "-" "Mozilla/5.0 (X11; Linux x86_64)"`, nowStr),
		fmt.Sprintf(`192.168.1.12 - - [%s] "GET /api/v1/status HTTP/1.1" 200 234 "-" "curl/7.68.0"`, nowStr),
		fmt.Sprintf(`192.168.1.15 - - [%s] "POST /login HTTP/1.1" 200 890 "https://example.com/login" "Mozilla/5.0"`, nowStr),
		fmt.Sprintf(`192.168.1.15 - - [%s] "POST /login HTTP/1.1" 401 120 "https://example.com/login" "Mozilla/5.0"`, nowStr),
		fmt.Sprintf(`45.33.32.156 - - [%s] "GET /wp-admin HTTP/1.1" 404 162 "-" "Go-http-client/1.1"`, nowStr),
		fmt.Sprintf(`45.33.32.156 - - [%s] "GET /wp-login.php HTTP/1.1" 404 162 "-" "Go-http-client/1.1"`, nowStr),
		fmt.Sprintf(`45.33.32.156 - - [%s] "GET /.env HTTP/1.1" 404 162 "-" "Python-urllib/3.8"`, nowStr),
		fmt.Sprintf(`45.33.32.156 - - [%s] "GET /phpmyadmin HTTP/1.1" 404 162 "-" "Python-urllib/3.8"`, nowStr),
		fmt.Sprintf(`192.168.1.20 - - [%s] "GET /api/v1/data HTTP/1.1" 500 532 "-" "Mozilla/5.0"`, nowStr),
		fmt.Sprintf(`192.168.1.10 - - [%s] "GET /dashboard HTTP/1.1" 200 12500 "-" "Mozilla/5.0"`, nowStr),
	}

	content := strings.Join(sampleLogs, "\n") + "\n"
	return os.WriteFile(filePath, []byte(content), 0644)
}

func AnalyzeLogs(filePath string) (*LogSummary, error) {
	if err := EnsureSampleLog(filePath); err != nil {
		// Proceed even if creating sample fails
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}
	defer file.Close()

	summary := &LogSummary{
		StatusCounts:     make(map[int]int),
		RouteCounts:      make(map[string]int),
		IPCounts:         make(map[string]int),
		SuspiciousRoutes: make(map[string]int),
	}

	var minTime, maxTime time.Time
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		entry, err := ParseLine(line)
		if err != nil {
			continue
		}

		summary.TotalRequests++
		summary.StatusCounts[entry.Status]++
		summary.RouteCounts[entry.Route]++
		summary.IPCounts[entry.IP]++

		if minTime.IsZero() || entry.Timestamp.Before(minTime) {
			minTime = entry.Timestamp
		}
		if maxTime.IsZero() || entry.Timestamp.After(maxTime) {
			maxTime = entry.Timestamp
		}

		if entry.Status >= 400 {
			summary.ErrorCount++
			summary.ErrorEntries = append(summary.ErrorEntries, *entry)
		}

		if entry.Status == 401 || entry.Status == 403 || (strings.Contains(entry.Route, "/login") && entry.Status >= 400) {
			summary.FailedLogins++
		}

		lowerRoute := strings.ToLower(entry.Route)
		for _, kw := range suspiciousKeywords {
			if strings.Contains(lowerRoute, kw) {
				summary.SuspiciousRoutes[entry.Route]++
				break
			}
		}

		summary.RecentEntries = append(summary.RecentEntries, *entry)
	}

	// Calculate requests/min
	durationMin := maxTime.Sub(minTime).Minutes()
	if durationMin > 0 {
		summary.RequestsPerMin = float64(summary.TotalRequests) / durationMin
	} else {
		summary.RequestsPerMin = float64(summary.TotalRequests)
	}

	// Top routes
	var routes []RouteTraffic
	for r, c := range summary.RouteCounts {
		routes = append(routes, RouteTraffic{Route: r, Count: c})
	}
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Count > routes[j].Count
	})
	if len(routes) > 5 {
		summary.TopRoutes = routes[:5]
	} else {
		summary.TopRoutes = routes
	}

	// Top IPs
	type ipCount struct {
		ip    string
		count int
	}
	var ips []ipCount
	for ip, c := range summary.IPCounts {
		ips = append(ips, ipCount{ip: ip, count: c})
	}
	sort.Slice(ips, func(i, j int) bool {
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

func TailLogs(filePath string, limit int, errorsOnly bool, routesOnly bool) ([]LogEntry, map[string]int, error) {
	if err := EnsureSampleLog(filePath); err != nil {
		// Ignore
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to open log file: %w", err)
	}
	defer file.Close()

	var entries []LogEntry
	routeCounts := make(map[string]int)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		entry, err := ParseLine(line)
		if err != nil {
			continue
		}

		routeCounts[entry.Route]++

		if errorsOnly && entry.Status < 400 {
			continue
		}

		entries = append(entries, *entry)
	}

	if limit > 0 && len(entries) > limit {
		entries = entries[len(entries)-limit:]
	}

	return entries, routeCounts, nil
}
