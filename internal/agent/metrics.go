package agent

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/dockpilot/dockpilot/internal/protocol"
)

type MetricsCollector struct {
	lastIdle  uint64
	lastTotal uint64
}

func (m *MetricsCollector) Collect(containerCount int) protocol.MetricsPayload {
	idle, total := readCPU()
	cpuPercent := 0.0
	if m.lastTotal > 0 && total > m.lastTotal {
		totalDelta := total - m.lastTotal
		idleDelta := idle - m.lastIdle
		if totalDelta > 0 {
			cpuPercent = (1.0 - float64(idleDelta)/float64(totalDelta)) * 100
		}
	}
	m.lastIdle = idle
	m.lastTotal = total

	memUsed, memTotal := readMemory()
	diskUsed, diskTotal := readDisk("/")
	rx, tx := readNetwork()
	return protocol.MetricsPayload{
		CPUPercent:    cpuPercent,
		MemoryUsed:    memUsed,
		MemoryTotal:   memTotal,
		DiskUsed:      diskUsed,
		DiskTotal:     diskTotal,
		NetworkRx:     rx,
		NetworkTx:     tx,
		ContainerCount: containerCount,
	}
}

func readCPU() (idle, total uint64) {
	file, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		return 0, 0
	}
	parts := strings.Fields(scanner.Text())
	for i, part := range parts[1:] {
		value, _ := strconv.ParseUint(part, 10, 64)
		total += value
		if i == 3 || i == 4 {
			idle += value
		}
	}
	return idle, total
}

func readMemory() (used, total uint64) {
	file, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	values := map[string]uint64{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 2 {
			key := strings.TrimSuffix(parts[0], ":")
			value, _ := strconv.ParseUint(parts[1], 10, 64)
			values[key] = value * 1024
		}
	}
	total = values["MemTotal"]
	available := values["MemAvailable"]
	if total > available {
		used = total - available
	}
	return used, total
}

func readDisk(path string) (used, total uint64) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, 0
	}
	total = stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	if total > free {
		used = total - free
	}
	return used, total
}

func readNetwork() (rx, tx uint64) {
	file, err := os.Open("/proc/net/dev")
	if err != nil {
		return 0, 0
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}
		nameAndRest := strings.SplitN(line, ":", 2)
		if len(nameAndRest) != 2 || strings.TrimSpace(nameAndRest[0]) == "lo" {
			continue
		}
		fields := strings.Fields(nameAndRest[1])
		if len(fields) >= 16 {
			v, _ := strconv.ParseUint(fields[0], 10, 64)
			rx += v
			v, _ = strconv.ParseUint(fields[8], 10, 64)
			tx += v
		}
	}
	return rx, tx
}
