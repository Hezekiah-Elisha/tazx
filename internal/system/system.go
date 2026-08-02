package system

import (
	"fmt"
	"sort"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type SystemStatus struct {
	CPUPercent    float64
	MemoryTotalGB float64
	MemoryUsedGB  float64
	MemoryFreeGB  float64
	MemoryPercent float64
	DiskTotalGB   float64
	DiskUsedGB    float64
	DiskFreeGB    float64
	DiskPercent   float64
	DiskStatus    string
	HostName      string
	Platform      string
	UptimeSeconds uint64
	CPUInfo       []cpu.InfoStat
	ActiveUsers   []host.UserStat
}

type ProcessInfo struct {
	PID        int32
	Name       string
	Username   string
	CPUPercent float64
	MemPercent float32
	Status     string
}

func GetSystemStatus() (*SystemStatus, error) {
	status := &SystemStatus{}

	// CPU
	cpuPercents, err := cpu.Percent(100*time.Millisecond, false)
	if err == nil && len(cpuPercents) > 0 {
		status.CPUPercent = cpuPercents[0]
	}

	cpuInfos, err := cpu.Info()
	if err == nil {
		status.CPUInfo = cpuInfos
	}

	// Memory
	vMem, err := mem.VirtualMemory()
	if err == nil {
		status.MemoryTotalGB = float64(vMem.Total) / (1024 * 1024 * 1024)
		status.MemoryUsedGB = float64(vMem.Used) / (1024 * 1024 * 1024)
		status.MemoryFreeGB = float64(vMem.Free) / (1024 * 1024 * 1024)
		status.MemoryPercent = vMem.UsedPercent
	}

	// Disk
	dUsage, err := disk.Usage("/")
	if err == nil {
		status.DiskTotalGB = float64(dUsage.Total) / (1024 * 1024 * 1024)
		status.DiskUsedGB = float64(dUsage.Used) / (1024 * 1024 * 1024)
		status.DiskFreeGB = float64(dUsage.Free) / (1024 * 1024 * 1024)
		status.DiskPercent = dUsage.UsedPercent

		if dUsage.UsedPercent > 90 {
			status.DiskStatus = "CRITICAL"
		} else if dUsage.UsedPercent > 80 {
			status.DiskStatus = "WARNING"
		} else {
			status.DiskStatus = "OK"
		}
	} else {
		status.DiskStatus = "UNKNOWN"
	}

	// Host
	hInfo, err := host.Info()
	if err == nil {
		status.HostName = hInfo.Hostname
		status.Platform = fmt.Sprintf("%s %s", hInfo.Platform, hInfo.PlatformVersion)
		status.UptimeSeconds = hInfo.Uptime
	}

	// Users
	uStats, err := host.Users()
	if err == nil {
		status.ActiveUsers = uStats
	}

	return status, nil
}

func GetTopProcesses(limit int, sortByMemory bool) ([]ProcessInfo, error) {
	procs, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var procList []ProcessInfo
	for _, p := range procs {
		name, err := p.Name()
		if err != nil || name == "" {
			continue
		}
		user, _ := p.Username()
		cpuP, _ := p.CPUPercent()
		memP, _ := p.MemoryPercent()
		statusSlice, _ := p.Status()
		pStatus := "R"
		if len(statusSlice) > 0 {
			pStatus = statusSlice[0]
		}

		procList = append(procList, ProcessInfo{
			PID:        p.Pid,
			Name:       name,
			Username:   user,
			CPUPercent: cpuP,
			MemPercent: memP,
			Status:     pStatus,
		})
	}

	if sortByMemory {
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].MemPercent > procList[j].MemPercent
		})
	} else {
		sort.Slice(procList, func(i, j int) bool {
			return procList[i].CPUPercent > procList[j].CPUPercent
		})
	}

	if len(procList) > limit {
		procList = procList[:limit]
	}

	return procList, nil
}
