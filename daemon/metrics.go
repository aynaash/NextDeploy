package daemon

func CollectSystemMetrics() SystemMetrics {
	// TODO: Parse /proc/ or use gopsutil
	return SystemMetrics{
		CPUUsage:    12.3,
		MemoryUsage: 57.8,
		DiskUsage:   42.1,
		Uptime:      "3h17m",
	}
}
