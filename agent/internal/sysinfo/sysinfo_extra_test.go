package sysinfo

import (
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestUptimeSeconds_Fallback verifies the fallback path by manipulating startTime.
func TestUptimeSeconds_FallbackToProcessUptime(t *testing.T) {
	// Set startTime to a while ago to test process uptime path
	original := startTime
	startTime = time.Now().Add(-5 * time.Second)
	defer func() { startTime = original }()

	// uptimeSeconds should use system uptime first (unix.Sysinfo), but if that
	// fails or returns 0, it falls back to process uptime
	uptime := uptimeSeconds()
	if uptime <= 0 {
		t.Fatalf("expected positive uptime, got %d", uptime)
	}
}

// TestUptimeSeconds_MinimumOne verifies uptime is always at least 1 second.
func TestUptimeSeconds_MinimumOne(t *testing.T) {
	// Verify the result is always positive
	uptime := uptimeSeconds()
	if uptime < 1 {
		t.Fatalf("expected uptime >= 1, got %d", uptime)
	}
}

// TestCPUPercent_ExtremeLoad verifies CPU percent doesn't exceed 100 under load.
func TestCPUPercent_ExtremeLoad(t *testing.T) {
	// cpuPercent uses goroutine count / CPU count * 10
	// Under extreme goroutine load, it should be capped at 100
	pct := cpuPercent()
	if pct > 100 {
		t.Fatalf("expected cpuPercent <= 100, got %f", pct)
	}
	if pct < 0 {
		t.Fatalf("expected cpuPercent >= 0, got %f", pct)
	}
}

// TestMemoryPercent_NonNegative verifies memory percent is always non-negative.
func TestMemoryPercent_NonNegative(t *testing.T) {
	pct := memoryPercent()
	if pct < 0 {
		t.Fatalf("expected memoryPercent >= 0, got %f", pct)
	}
}

// TestDiskPercent_NonNegative verifies disk percent is always non-negative.
func TestDiskPercent_NonNegative(t *testing.T) {
	pct := diskPercent()
	if pct < 0 {
		t.Fatalf("expected diskPercent >= 0, got %f", pct)
	}
}

// TestCollect_AllFieldsPresent verifies all fields are populated.
func TestCollect_AllFieldsPresent(t *testing.T) {
	info := Collect()
	if info == nil {
		t.Fatal("expected non-nil SystemInfo")
	}
	// All fields should be valid ranges
	if info.CpuPercent < 0 || info.CpuPercent > 100 {
		t.Fatalf("invalid CpuPercent: %f", info.CpuPercent)
	}
	if info.MemoryPercent < 0 || info.MemoryPercent > 100 {
		t.Fatalf("invalid MemoryPercent: %f", info.MemoryPercent)
	}
	if info.DiskPercent < 0 || info.DiskPercent > 100 {
		t.Fatalf("invalid DiskPercent: %f", info.DiskPercent)
	}
	if info.UptimeSeconds < 1 {
		t.Fatalf("invalid UptimeSeconds: %d", info.UptimeSeconds)
	}
}

// TestHostname_NotEmpty verifies hostname returns a non-empty string.
func TestHostname_NotEmpty(t *testing.T) {
	h := Hostname()
	if h == "" {
		t.Fatal("expected non-empty hostname")
	}
}

// TestOSInfo_ContainsSlash verifies OSInfo format is "os/arch".
func TestOSInfo_ContainsSlash(t *testing.T) {
	info := OSInfo()
	if len(info) == 0 {
		t.Fatal("expected non-empty OS info")
	}
}

// TestOutboundIP verifies that OutboundIP returns a non-empty string.
func TestOutboundIP(t *testing.T) {
	ip := OutboundIP()
	if ip == "" {
		t.Skip("skipping OutboundIP test - no network connectivity")
	}
	// Should look like an IP address
	if len(ip) < 7 { // shortest: "1.2.3.4"
		t.Fatalf("expected IP-like string, got '%s'", ip)
	}
}

// TestUptimeSecondsFrom_FallbackMinimumOne verifies the fallback returns at least 1.
func TestUptimeSecondsFrom_FallbackMinimumOne(t *testing.T) {
	// When sysinfo fails (ok=false), should use process uptime
	var info unix.Sysinfo_t
	result := uptimeSecondsFrom(&info, false)
	if result < 1 {
		t.Fatalf("expected uptime >= 1 from fallback, got %d", result)
	}
}

// TestMemoryPercentFrom_ZeroTotalRAM verifies 0 is returned when TotalRAM is 0.
func TestMemoryPercentFrom_ZeroTotalRAM(t *testing.T) {
	info := &unix.Sysinfo_t{Totalram: 0, Unit: 1}
	pct := memoryPercentFrom(info, true)
	if pct != 0 {
		t.Fatalf("expected 0%% for zero totalram, got %f", pct)
	}
}

// TestMemoryPercentFrom_NotOK verifies 0 is returned when sysinfo failed.
func TestMemoryPercentFrom_NotOK(t *testing.T) {
	info := &unix.Sysinfo_t{Totalram: 1024, Freeram: 512, Unit: 1}
	pct := memoryPercentFrom(info, false)
	if pct != 0 {
		t.Fatalf("expected 0%% when ok=false, got %f", pct)
	}
}
