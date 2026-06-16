package docker

// CPUPercent applies the standard Docker formula:
// (cpuDelta / systemDelta) * onlineCPUs * 100.
// All totals are cumulative nanosecond counters from the stats stream.
func CPUPercent(cpuTotal, preCPUTotal, systemUsage, preSystemUsage uint64, onlineCPUs uint32) float64 {
	cpuDelta := float64(cpuTotal) - float64(preCPUTotal)
	sysDelta := float64(systemUsage) - float64(preSystemUsage)
	if sysDelta <= 0 || cpuDelta < 0 || onlineCPUs == 0 {
		return 0
	}
	return (cpuDelta / sysDelta) * float64(onlineCPUs) * 100.0
}
