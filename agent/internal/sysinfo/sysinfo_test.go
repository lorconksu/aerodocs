package sysinfo

import "testing"

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
