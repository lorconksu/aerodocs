package sysinfo

import (
	"strings"
	"testing"
)

func TestCollect(t *testing.T) {
	info := Collect()
	if info.CpuPercent < 0 || info.CpuPercent > 100 {
		t.Fatalf("cpu_percent out of range: %f", info.CpuPercent)
	}
	if info.MemoryPercent < 0 || info.MemoryPercent > 100 {
		t.Fatalf("memory_percent out of range: %f", info.MemoryPercent)
	}
	if info.DiskPercent < 0 || info.DiskPercent > 100 {
		t.Fatalf("disk_percent out of range: %f", info.DiskPercent)
	}
	if info.UptimeSeconds <= 0 {
		t.Fatalf("uptime_seconds should be positive, got %d", info.UptimeSeconds)
	}
}

func TestHostname(t *testing.T) {
	h := Hostname()
	if h == "" {
		t.Fatal("expected non-empty hostname")
	}
}

func TestOSInfo(t *testing.T) {
	info := OSInfo()
	if !strings.Contains(info, "/") {
		t.Fatalf("expected 'os/arch' format, got '%s'", info)
	}
}

func TestCPUPercent_Bounded(t *testing.T) {
	pct := cpuPercent()
	if pct < 0 || pct > 100 {
		t.Fatalf("cpuPercent out of range: %f", pct)
	}
}

func TestMemoryPercent_Bounded(t *testing.T) {
	pct := memoryPercent()
	if pct < 0 || pct > 100 {
		t.Fatalf("memoryPercent out of range: %f", pct)
	}
}

func TestDiskPercent_Bounded(t *testing.T) {
	pct := diskPercent()
	if pct < 0 || pct > 100 {
		t.Fatalf("diskPercent out of range: %f", pct)
	}
}

func TestUptimeSeconds_Positive(t *testing.T) {
	uptime := uptimeSeconds()
	if uptime <= 0 {
		t.Fatalf("expected positive uptime, got %d", uptime)
	}
}
