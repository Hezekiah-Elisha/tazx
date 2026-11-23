package system

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

const bytesPerGB = 1024 * 1024 * 1024

// CPUCoreInfo describes logical/physical CPU hardware metadata.
type CPUCoreInfo struct {
	Index     int     `json:"index"`
	ModelName string  `json:"model_name"`
	Cores     int32   `json:"cores"`
	Mhz       float64 `json:"mhz"`
}

// UserSession describes an active logged-in user session.
type UserSession struct {
	User     string `json:"user"`
	Terminal string `json:"terminal"`
	Host     string `json:"host"`
	Started  int64  `json:"started"`
}

// SystemStatus aggregates core server metrics.
type SystemStatus struct {
	CPUPercent     float64   `json:"cpu_percent"`
	PerCorePercent []float64 `json:"per_core_percent,omitempty"`
	PhysicalCores  int       `json:"physical_cores"`
	LogicalCores   int       `json:"logical_cores"`

	LoadAvg1         float64 `json:"load_avg_1"`
	LoadAvg5         float64 `json:"load_avg_5"`
	LoadAvg15        float64 `json:"load_avg_15"`
	LoadAvgAvailable bool    `json:"load_avg_available"`

	MemoryTotalBytes     uint64  `json:"memory_total_bytes"`
	MemoryUsedBytes      uint64  `json:"memory_used_bytes"`
	MemoryAvailableBytes uint64  `json:"memory_available_bytes"`
	MemoryFreeBytes      uint64  `json:"memory_free_bytes"`
	MemoryTotalGB        float64 `json:"memory_total_gb"`
	MemoryUsedGB         float64 `json:"memory_used_gb"`
	MemoryAvailableGB    float64 `json:"memory_available_gb"`
	MemoryFreeGB         float64 `json:"memory_free_gb"`
	MemoryPercent        float64 `json:"memory_percent"`

	SwapTotalBytes uint64  `json:"swap_total_bytes"`
	SwapUsedBytes  uint64  `json:"swap_used_bytes"`
	SwapFreeBytes  uint64  `json:"swap_free_bytes"`
	SwapTotalGB    float64 `json:"swap_total_gb"`
	SwapUsedGB     float64 `json:"swap_used_gb"`
	SwapFreeGB     float64 `json:"swap_free_gb"`
	SwapPercent    float64 `json:"swap_percent"`
	SwapAvailable  bool    `json:"swap_available"`

	DiskTotalGB   float64 `json:"disk_total_gb"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskFreeGB    float64 `json:"disk_free_gb"`
	DiskPercent   float64 `json:"disk_percent"`
	DiskStatus    string  `json:"disk_status"`
	HostName      string  `json:"hostname"`
	Platform      string  `json:"platform"`
	KernelVersion string  `json:"kernel_version"`
	OS            string  `json:"os"`
	UptimeSeconds uint64  `json:"uptime_seconds"`

	CPUInfo     []CPUCoreInfo `json:"cpu_info,omitempty"`
	ActiveUsers []UserSession `json:"active_users,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
}

// ProcessInfo represents a single OS process snapshot.
type ProcessInfo struct {
	PID        int32   `json:"pid"`
	Name       string  `json:"name"`
	Username   string  `json:"username"`
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float32 `json:"memory_percent"`
	Status     string  `json:"status"`
}

// DiskStatusFor classifies a disk usage percentage against warnThreshold.
func DiskStatusFor(percent, warnThreshold float64) string {
	critThreshold := 90.0
	if warnThreshold > 0 && warnThreshold+5.0 > critThreshold {
		critThreshold = warnThreshold + 5.0
		if critThreshold > 95.0 {
			critThreshold = 95.0
		}
	}
	if percent >= critThreshold {
		return "CRITICAL"
	}
	if warnThreshold > 0 && percent >= warnThreshold {
		return "WARNING"
	}
	return "OK"
}

// GetSystemStatus collects CPU, memory, swap, root disk, host, and session metrics.
func GetSystemStatus(ctx context.Context) (*SystemStatus, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	status := &SystemStatus{}
	coreFailures := 0

	// CPU aggregate usage over a 250ms sample window.
	cpuPercents, err := cpu.PercentWithContext(ctx, 250*time.Millisecond, false)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		coreFailures++
		status.Warnings = append(status.Warnings, fmt.Sprintf("CPU usage unavailable: %v", err))
	} else if len(cpuPercents) > 0 {
		status.CPUPercent = cpuPercents[0]
	}

	if phys, err := cpu.CountsWithContext(ctx, false); err == nil {
		status.PhysicalCores = phys
	}
	if logical, err := cpu.CountsWithContext(ctx, true); err == nil {
		status.LogicalCores = logical
	}

	cpuInfos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("CPU hardware info unavailable: %v", err))
	} else {
		for idx, info := range cpuInfos {
			status.CPUInfo = append(status.CPUInfo, CPUCoreInfo{
				Index:     idx,
				ModelName: info.ModelName,
				Cores:     info.Cores,
				Mhz:       info.Mhz,
			})
		}
	}

	if avg, err := load.AvgWithContext(ctx); err == nil && avg != nil {
		status.LoadAvg1 = avg.Load1
		status.LoadAvg5 = avg.Load5
		status.LoadAvg15 = avg.Load15
		status.LoadAvgAvailable = true
	} else if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("Load average unavailable: %v", err))
	}

	// Virtual Memory (uses Available rather than Free so Linux page cache is accounted for).
	vMem, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		coreFailures++
		status.Warnings = append(status.Warnings, fmt.Sprintf("Memory stats unavailable: %v", err))
	} else if vMem != nil {
		status.MemoryTotalBytes = vMem.Total
		status.MemoryUsedBytes = vMem.Used
		status.MemoryAvailableBytes = vMem.Available
		status.MemoryFreeBytes = vMem.Free
		status.MemoryTotalGB = float64(vMem.Total) / bytesPerGB
		status.MemoryUsedGB = float64(vMem.Used) / bytesPerGB
		status.MemoryAvailableGB = float64(vMem.Available) / bytesPerGB
		status.MemoryFreeGB = float64(vMem.Free) / bytesPerGB
		status.MemoryPercent = vMem.UsedPercent
	}

	// Swap Memory
	swapMem, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("Swap stats unavailable: %v", err))
	} else if swapMem != nil {
		status.SwapAvailable = true
		status.SwapTotalBytes = swapMem.Total
		status.SwapUsedBytes = swapMem.Used
		status.SwapFreeBytes = swapMem.Free
		status.SwapTotalGB = float64(swapMem.Total) / bytesPerGB
		status.SwapUsedGB = float64(swapMem.Used) / bytesPerGB
		status.SwapFreeGB = float64(swapMem.Free) / bytesPerGB
		status.SwapPercent = swapMem.UsedPercent
	}

	// Root Disk Usage
	dUsage, err := disk.UsageWithContext(ctx, "/")
	if err != nil {
		coreFailures++
		status.DiskStatus = "UNKNOWN"
		status.Warnings = append(status.Warnings, fmt.Sprintf("Root disk usage unavailable: %v", err))
	} else if dUsage != nil {
		status.DiskTotalGB = float64(dUsage.Total) / bytesPerGB
		status.DiskUsedGB = float64(dUsage.Used) / bytesPerGB
		status.DiskFreeGB = float64(dUsage.Free) / bytesPerGB
		status.DiskPercent = dUsage.UsedPercent
		status.DiskStatus = DiskStatusFor(dUsage.UsedPercent, 85.0)
	}

	if coreFailures == 3 {
		return nil, fmt.Errorf("failed to collect core system metrics: %s", status.Warnings[0])
	}

	// Host Info
	hInfo, err := host.InfoWithContext(ctx)
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("Host info unavailable: %v", err))
	} else if hInfo != nil {
		status.HostName = hInfo.Hostname
		status.Platform = fmt.Sprintf("%s %s", hInfo.Platform, hInfo.PlatformVersion)
		status.KernelVersion = hInfo.KernelVersion
		status.OS = hInfo.OS
		status.UptimeSeconds = hInfo.Uptime
	}

	// Active Logged-In Users
	uStats, err := host.UsersWithContext(ctx)
	if err != nil {
		status.Warnings = append(status.Warnings, fmt.Sprintf("Active user sessions unavailable: %v", err))
	} else {
		for _, u := range uStats {
			status.ActiveUsers = append(status.ActiveUsers, UserSession{
				User:     u.User,
				Terminal: u.Terminal,
				Host:     u.Host,
				Started:  int64(u.Started),
			})
		}
	}

	return status, nil
}

// GetPerCoreCPUUsage samples per-logical-core CPU utilization over 250ms.
func GetPerCoreCPUUsage(ctx context.Context) ([]float64, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	return cpu.PercentWithContext(ctx, 250*time.Millisecond, true)
}

// ProcessSampler caches process handles across samples so CPU% reflects
// interval usage instead of process-lifetime average, pruning exited PIDs.
type ProcessSampler struct {
	mu    sync.Mutex
	cache map[int32]*process.Process
}

// NewProcessSampler returns an initialized ProcessSampler.
func NewProcessSampler() *ProcessSampler {
	return &ProcessSampler{
		cache: make(map[int32]*process.Process),
	}
}

// Sample collects and sorts running processes by CPU or memory usage.
func (ps *ProcessSampler) Sample(ctx context.Context, limit int, sortByMemory bool) ([]ProcessInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, err
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	coldStart := len(ps.cache) == 0
	live := make(map[int32]*process.Process, len(procs))
	for _, p := range procs {
		if existing, ok := ps.cache[p.Pid]; ok {
			live[p.Pid] = existing
		} else {
			// Seed initial CPU sample baseline for newly seen processes.
			_, _ = p.PercentWithContext(ctx, 0)
			live[p.Pid] = p
		}
	}
	ps.cache = live

	if coldStart && !sortByMemory {
		// Wait briefly so one-shot CPU samples reflect recent activity instead of 0.
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}

	procList := make([]ProcessInfo, 0, len(live))
	for pid, p := range live {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		name, err := p.NameWithContext(ctx)
		if err != nil || name == "" {
			continue
		}
		user, err := p.UsernameWithContext(ctx)
		if err != nil || user == "" {
			user = "-"
		}
		cpuP, _ := p.PercentWithContext(ctx, 0)
		memP, _ := p.MemoryPercentWithContext(ctx)
		statusSlice, _ := p.StatusWithContext(ctx)
		pStatus := "?"
		if len(statusSlice) > 0 && statusSlice[0] != "" {
			pStatus = statusSlice[0]
		}

		procList = append(procList, ProcessInfo{
			PID:        pid,
			Name:       name,
			Username:   user,
			CPUPercent: cpuP,
			MemPercent: memP,
			Status:     pStatus,
		})
	}

	if sortByMemory {
		sort.Slice(procList, func(i, j int) bool {
			if procList[i].MemPercent == procList[j].MemPercent {
				return procList[i].PID < procList[j].PID
			}
			return procList[i].MemPercent > procList[j].MemPercent
		})
	} else {
		sort.Slice(procList, func(i, j int) bool {
			if procList[i].CPUPercent == procList[j].CPUPercent {
				return procList[i].PID < procList[j].PID
			}
			return procList[i].CPUPercent > procList[j].CPUPercent
		})
	}

	if limit > 0 && len(procList) > limit {
		procList = procList[:limit]
	}
	return procList, nil
}

// GetTopProcesses runs a one-shot process sample returning the top limit processes.
func GetTopProcesses(ctx context.Context, limit int, sortByMemory bool) ([]ProcessInfo, error) {
	return NewProcessSampler().Sample(ctx, limit, sortByMemory)
}
