package sysinfo

import (
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/unix"

	pb "github.com/wyiu/aerodocs/proto/aerodocs/v1"
)

var startTime = time.Now()

func Collect() *pb.SystemInfo {
	return &pb.SystemInfo{
		CpuPercent:    cpuPercent(),
		MemoryPercent: memoryPercent(),
		DiskPercent:   diskPercent(),
		UptimeSeconds: uptimeSeconds(),
	}
}

func uptimeSeconds() int64 {
	// Try system uptime first via unix.Sysinfo (always > 0 on a running system)
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err == nil && info.Uptime > 0 {
		return info.Uptime
	}
	// Fall back to process uptime, minimum 1 second
	secs := int64(time.Since(startTime).Seconds())
	if secs < 1 {
		secs = 1
	}
	return secs
}

func cpuPercent() float64 {
	numCPU := runtime.NumCPU()
	numGoroutine := runtime.NumGoroutine()
	pct := float64(numGoroutine) / float64(numCPU) * 10.0
	if pct > 100 {
		pct = 100
	}
	return pct
}

func memoryPercent() float64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	var info unix.Sysinfo_t
	if err := unix.Sysinfo(&info); err != nil {
		return 0
	}
	totalBytes := info.Totalram * uint64(info.Unit)
	if totalBytes == 0 {
		return 0
	}
	usedBytes := totalBytes - (info.Freeram * uint64(info.Unit))
	return float64(usedBytes) / float64(totalBytes) * 100.0
}

func diskPercent() float64 {
	var stat unix.Statfs_t
	path := "/"
	if err := unix.Statfs(path, &stat); err != nil {
		return 0
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	if total == 0 {
		return 0
	}
	used := total - free
	return float64(used) / float64(total) * 100.0
}

func Hostname() string {
	h, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return h
}

func OSInfo() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}
