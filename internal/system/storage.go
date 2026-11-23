package system

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/disk"
)

// DiskPartition describes a mounted filesystem and its capacity statistics.
type DiskPartition struct {
	Filesystem     string  `json:"filesystem"`
	FSType         string  `json:"fs_type"`
	Mountpoint     string  `json:"mountpoint"`
	TotalBytes     uint64  `json:"total_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	Status         string  `json:"status"`
}

// DirUsage holds the calculated byte size of a directory subtree.
type DirUsage struct {
	Path      string `json:"path"`
	SizeBytes uint64 `json:"size_bytes"`
}

var ignoredFSTypes = map[string]bool{
	"autofs":     true,
	"bpf":        true,
	"cgroup":     true,
	"cgroup2":    true,
	"configfs":   true,
	"debugfs":    true,
	"devfs":      true,
	"devpts":     true,
	"devtmpfs":   true,
	"efivarfs":   true,
	"fusectl":    true,
	"hugetlbfs":  true,
	"mqueue":     true,
	"nsfs":       true,
	"nullfs":     true,
	"overlay":    true,
	"proc":       true,
	"pstore":     true,
	"ramfs":      true,
	"securityfs": true,
	"squashfs":   true,
	"sysfs":      true,
	"tmpfs":      true,
	"tracefs":    true,
}

// GetDiskPartitions returns mounted filesystems with capacity and status.
// When includeAll is false, pseudo/virtual filesystems and snap mounts are omitted.
func GetDiskPartitions(ctx context.Context, includeAll bool, warnThreshold float64) ([]DiskPartition, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	parts, err := disk.PartitionsWithContext(ctx, includeAll)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list disk partitions: %w", err)
	}

	seenMounts := make(map[string]bool)
	var result []DiskPartition
	var warnings []string

	for _, p := range parts {
		if ctx.Err() != nil {
			return nil, warnings, ctx.Err()
		}
		if !includeAll && p.Mountpoint != "/" {
			if ignoredFSTypes[strings.ToLower(p.Fstype)] {
				continue
			}
			if strings.HasPrefix(p.Mountpoint, "/snap/") {
				continue
			}
		}
		if seenMounts[p.Mountpoint] {
			continue
		}
		seenMounts[p.Mountpoint] = true

		usage, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("Skipped mount %s: %v", p.Mountpoint, err))
			continue
		}
		if !includeAll && usage.Total == 0 {
			continue
		}

		avail := usage.Free
		result = append(result, DiskPartition{
			Filesystem:     p.Device,
			FSType:         p.Fstype,
			Mountpoint:     p.Mountpoint,
			TotalBytes:     usage.Total,
			UsedBytes:      usage.Used,
			AvailableBytes: avail,
			UsedPercent:    usage.UsedPercent,
			Status:         DiskStatusFor(usage.UsedPercent, warnThreshold),
		})
	}

	// In container/overlay/sandbox environments, Partitions(false) may omit "/".
	if !seenMounts["/"] {
		if usage, uErr := disk.UsageWithContext(ctx, "/"); uErr == nil && usage != nil && usage.Total > 0 {
			fsDevice := "rootfs"
			fsType := usage.Fstype
			if allParts, allErr := disk.PartitionsWithContext(ctx, true); allErr == nil {
				for _, ap := range allParts {
					if ap.Mountpoint == "/" {
						if ap.Device != "" {
							fsDevice = ap.Device
						}
						if ap.Fstype != "" {
							fsType = ap.Fstype
						}
						break
					}
				}
			}
			result = append(result, DiskPartition{
				Filesystem:     fsDevice,
				FSType:         fsType,
				Mountpoint:     "/",
				TotalBytes:     usage.Total,
				UsedBytes:      usage.Used,
				AvailableBytes: usage.Free,
				UsedPercent:    usage.UsedPercent,
				Status:         DiskStatusFor(usage.UsedPercent, warnThreshold),
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Mountpoint == "/" {
			return true
		}
		if result[j].Mountpoint == "/" {
			return false
		}
		return result[i].Mountpoint < result[j].Mountpoint
	})

	return result, warnings, nil
}

// FindLargeDirs performs a bounded, non-symlink-following scan under rootPath
// to identify the largest subdirectories up to maxDepth.
func FindLargeDirs(ctx context.Context, rootPath string, maxDepth int, limit int) ([]DirUsage, []string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.ContainsRune(rootPath, '\x00') || strings.TrimSpace(rootPath) == "" {
		return nil, nil, fmt.Errorf("invalid directory path")
	}
	if maxDepth < 1 {
		maxDepth = 1
	} else if maxDepth > 5 {
		maxDepth = 5
	}
	if limit < 1 {
		limit = 10
	} else if limit > 100 {
		limit = 100
	}

	cleaned := filepath.Clean(rootPath)
	linfo, err := os.Lstat(cleaned)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot inspect directory %s: %w", cleaned, err)
	}
	if linfo.Mode()&os.ModeSymlink != 0 {
		return nil, nil, fmt.Errorf("refusing to scan symlink path: %s", cleaned)
	}
	if !linfo.IsDir() {
		return nil, nil, fmt.Errorf("%s is not a directory", cleaned)
	}

	const maxVisitedEntries = 200000
	visited := 0
	permDenied := 0
	var warnings []string

	dirSizes := make(map[string]uint64)
	virtualRoots := map[string]bool{
		"/proc": true,
		"/sys":  true,
		"/dev":  true,
		"/run":  true,
	}

	cleanDepth := func(p string) int {
		if p == cleaned {
			return 0
		}
		rel, relErr := filepath.Rel(cleaned, p)
		if relErr != nil || rel == "." {
			return 0
		}
		return strings.Count(rel, string(filepath.Separator)) + 1
	}

	truncated := false
	walkErr := filepath.WalkDir(cleaned, func(p string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			if os.IsPermission(err) {
				permDenied++
				return fs.SkipDir
			}
			warnings = append(warnings, fmt.Sprintf("Skipped %s: %v", p, err))
			if d != nil && d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		// Never follow symlinks during disk scans.
		if d.Type()&os.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if p != cleaned && cleaned == "/" && d.IsDir() && virtualRoots[p] {
			return fs.SkipDir
		}

		visited++
		if visited > maxVisitedEntries {
			truncated = true
			return fs.SkipAll
		}

		if d.IsDir() {
			return nil
		}

		info, statErr := d.Info()
		if statErr != nil {
			return nil
		}
		size := uint64(info.Size())
		if size == 0 {
			return nil
		}

		// Attribute file size up to ancestor directories within maxDepth.
		dir := filepath.Dir(p)
		for {
			depth := cleanDepth(dir)
			if depth >= 1 && depth <= maxDepth {
				dirSizes[dir] += size
			}
			if dir == cleaned || dir == filepath.Dir(dir) {
				break
			}
			dir = filepath.Dir(dir)
		}
		return nil
	})
	if walkErr != nil && walkErr != fs.SkipAll {
		return nil, warnings, walkErr
	}

	if permDenied > 0 {
		warnings = append(warnings, fmt.Sprintf("Skipped %d entries due to insufficient permissions", permDenied))
	}
	if truncated {
		warnings = append(warnings, fmt.Sprintf("Scan stopped after %d entries to keep execution bounded; narrow --dir or --depth for exact results", maxVisitedEntries))
	}

	results := make([]DirUsage, 0, len(dirSizes))
	for path, size := range dirSizes {
		results = append(results, DirUsage{
			Path:      path,
			SizeBytes: size,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].SizeBytes == results[j].SizeBytes {
			return results[i].Path < results[j].Path
		}
		return results[i].SizeBytes > results[j].SizeBytes
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, warnings, nil
}
