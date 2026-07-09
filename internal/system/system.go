// Package system samples host resource usage for the dashboard widgets.
package system

import (
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/sensors"
)

// Stats is a snapshot of host utilization, shaped for the CasaOS-style widgets.
type Stats struct {
	CPUPercent float64 `json:"cpu_percent"`
	CPUTempC   float64 `json:"cpu_temp_c"`
	MemPercent float64 `json:"mem_percent"`
	MemTotal   uint64  `json:"mem_total"`
	MemUsed    uint64  `json:"mem_used"`
	DiskPct    float64 `json:"disk_percent"`
	DiskTotal  uint64  `json:"disk_total"`
	DiskUsed   uint64  `json:"disk_used"`
}

// Collector samples utilization. CPU percentage is measured relative to the
// previous call, so keep one Collector and call Sample on a fixed interval.
type Collector struct {
	dataRoot string
}

// NewCollector returns a Collector reporting disk usage for dataRoot.
func NewCollector(dataRoot string) *Collector {
	if dataRoot == "" {
		dataRoot = "/"
	}
	// Prime the CPU counters so the first real Sample reflects an interval.
	_, _ = cpu.Percent(0, false)
	return &Collector{dataRoot: dataRoot}
}

// Sample returns a current utilization snapshot. Individual metrics degrade to
// zero rather than failing the whole call (e.g. temperature in a container).
func (c *Collector) Sample() Stats {
	var s Stats

	if pct, err := cpu.Percent(0, false); err == nil && len(pct) > 0 {
		s.CPUPercent = round1(pct[0])
	}
	if temps, err := sensors.SensorsTemperatures(); err == nil {
		s.CPUTempC = pickCPUTemp(temps)
	}
	if vm, err := mem.VirtualMemory(); err == nil {
		s.MemPercent = round1(vm.UsedPercent)
		s.MemTotal = vm.Total
		s.MemUsed = vm.Used
	}
	if du, err := disk.Usage(c.dataRoot); err == nil {
		s.DiskPct = round1(du.UsedPercent)
		s.DiskTotal = du.Total
		s.DiskUsed = du.Used
	}
	return s
}

func pickCPUTemp(temps []sensors.TemperatureStat) float64 {
	for _, t := range temps {
		switch t.SensorKey {
		case "coretemp_package_id_0", "cpu_thermal", "k10temp_tctl", "acpitz":
			if t.Temperature > 0 {
				return round1(t.Temperature)
			}
		}
	}
	// Fall back to the first sane reading, if any.
	for _, t := range temps {
		if t.Temperature > 0 && t.Temperature < 150 {
			return round1(t.Temperature)
		}
	}
	return 0
}

func round1(f float64) float64 {
	return float64(int64(f*10+0.5)) / 10
}
