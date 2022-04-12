// gonet is a simple go network tool that prints information about your system,
// cpu information, memory usage, disk usage, network interfaces & MAC address.
package gonet

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/jedib0t/go-pretty/table"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/net"
)

type sysMetrics struct {
	// Disk usage
	DiskSize  uint64
	DiskFree  uint64
	DiskUsage uint64

	// System Memory
	TotalMemory uint64
	FreeMemory  uint64
	UsedMemory  uint64
	CacheMemory uint64

	// CPU info
	GoNumCPU   int
	CPUInfo    []cpuinfo
	CPUPercent float64

	// host, platform
	Hostname         string
	RunningProcesses uint64
	Platform         string
	PlatformVersion  string

	// network identifiers
	MacAddr string
	IPAddrs map[string][]string
}

// Struct to hold cpu info
type cpuinfo struct {
	Index    int
	VendorID string
	Family   string
	Cores    int
	Model    string
	Speed    string
}

// getDiskUsage returns disk usage information
func getDiskUsage() (fs syscall.Statfs_t) {
	syscall.Statfs("/", &fs)
	return
}

// toHumanReadable converts bytes to human readable format
// e.g. 1.5 GB, 25 MB
func toHumanReadable(bytes uint64) string {
	// if less than 1 GiB, return in MB
	if bytes < 1024*1024*1024 {
		return fmt.Sprintf("%.2f MB", float64(bytes)/1024/1024)
	}

	return fmt.Sprintf("%.2f GB", float64(bytes)/1024/1024/1024)
}

// ReadMetrics reads metrics from the system
// and returns a Metrics struct
func ReadMetrics() sysMetrics {
	m := sysMetrics{}
	m.IPAddrs = make(map[string][]string)

	m.GoNumCPU = runtime.NumCPU()

	var memoryStats runtime.MemStats
	runtime.ReadMemStats(&memoryStats)

	// Disk usage
	fs := getDiskUsage()
	m.DiskSize = fs.Blocks * uint64(fs.Bsize)
	m.DiskFree = fs.Bfree * uint64(fs.Bsize)
	m.DiskUsage = m.DiskSize - m.DiskFree

	// System memory
	vmStat, err := mem.VirtualMemory()

	if err == nil {
		m.TotalMemory = vmStat.Total
		m.FreeMemory = vmStat.Free
		m.UsedMemory = vmStat.Used
		m.CacheMemory = vmStat.Cached
	}

	// cpu
	// loop through all available cpus
	cpuStats, err := cpu.Info()
	if err == nil {
		for index, c := range cpuStats {
			m.CPUInfo = append(m.CPUInfo, cpuinfo{
				Index:    index,
				VendorID: c.VendorID,
				Family:   c.Family,
				Cores:    int(c.Cores),
				Model:    c.ModelName,
				Speed:    strconv.FormatFloat(c.Mhz, 'f', 2, 64) + " MHz",
			})
		}
	}

	// cpu %
	percentage, err := cpu.Percent(0, false)
	if err == nil {
		m.CPUPercent = percentage[0]
	} else {
		fmt.Fprintf(os.Stderr, "error getting CPU Percent Usage: %s\n", err)
	}

	// hostStats
	hostStat, err := host.Info()
	if err == nil {
		m.Hostname = hostStat.Hostname
		m.RunningProcesses = hostStat.Procs
		m.Platform = hostStat.Platform
		m.PlatformVersion = hostStat.PlatformVersion
	}

	inetfStat, err := net.Interfaces()

	if err == nil && len(inetfStat) > 0 {
		for _, iface := range inetfStat {
			if iface.HardwareAddr != "" {
				m.MacAddr = iface.HardwareAddr
			}

			for _, addr := range iface.Addrs {
				m.IPAddrs[iface.Name] = append(m.IPAddrs[iface.Name], addr.Addr)
			}
		}
	}

	return m
}

// WriteMetrics writes metrics to the given writer.
// If writer is nil, it will write to stdout
func WriteMetrics(writer io.Writer) {
	if writer == nil {
		writer = os.Stdout
	}

	// Read the metrics
	metrics := ReadMetrics()
	fmt.Fprintln(writer)

	// print cpu metrics and usage
	t := table.NewWriter()
	t.SetOutputMirror(writer)
	t.AppendHeader(table.Row{"CPUs", "CPU Usage"})
	t.AppendRow(table.Row{metrics.GoNumCPU, fmt.Sprintf("%.2f%%", metrics.CPUPercent)})

	t.SetStyle(table.StyleColoredBlackOnBlueWhite)
	t.SetTitle("%s", "CPU Usage")
	t.Render()
	fmt.Fprintln(writer)

	// print architecture and stats for each cpu
	t1 := table.NewWriter()
	t1.SetTitle("%s", "CPU INFO")
	t1.SetOutputMirror(writer)
	t1.AppendHeader(table.Row{"#", "Vendor ID", "Family", "Cores", "Model", "Speed"})
	for _, c := range metrics.CPUInfo {
		t1.AppendRow(table.Row{
			c.Index, c.VendorID, c.Family, c.Cores, c.Model, c.Speed,
		})
	}

	t1.SetStyle(table.StyleColoredBright)
	t1.Render()
	fmt.Fprintln(writer)

	// print disk usage
	t2 := table.NewWriter()
	t2.SetTitle("%s", "Disk usage")
	t2.SetOutputMirror(writer)
	t2.AppendHeader(table.Row{"Disk Size", "Disk Free", "Disk Usage", "Disk Usage %"})
	t2.AppendRows([]table.Row{
		{toHumanReadable(metrics.DiskSize), toHumanReadable(metrics.DiskFree), toHumanReadable(metrics.DiskUsage),
			fmt.Sprintf("%.1f%%", float64(metrics.DiskUsage)/float64(metrics.DiskSize)*100)},
	})
	t2.SetStyle(table.StyleColoredBright)
	t2.Render()
	fmt.Fprintln(writer)

	// Print system memory usage
	t3 := table.NewWriter()
	t3.SetTitle("%s", "System Memory")
	t3.SetOutputMirror(writer)
	t3.AppendHeader(table.Row{"#", "Total Memory", "Free Memory", "Used Memory", "Cache Memory"})
	t3.AppendRows([]table.Row{
		{1, toHumanReadable(metrics.TotalMemory), toHumanReadable(metrics.FreeMemory), toHumanReadable(metrics.UsedMemory), toHumanReadable(metrics.CacheMemory)},
	})
	t3.SetStyle(table.StyleColoredBright)
	t3.Render()
	fmt.Fprintln(writer)

	// Print hostname, platform, platform version, running processes
	t4 := table.NewWriter()
	t4.SetTitle("%s", "Platform/System info:")
	t4.SetOutputMirror(writer)
	t4.AppendHeader(table.Row{"Hostname", "Running Processes", "Platform", "Platform Version"})
	t4.AppendRows([]table.Row{
		{metrics.Hostname, metrics.RunningProcesses, metrics.Platform, metrics.PlatformVersion},
	})
	t4.SetStyle(table.StyleColoredBright)
	t4.Render()
	fmt.Fprintln(writer)

	// Print MAC Address
	t5 := table.NewWriter()
	t5.SetTitle("%s", "Mac Address:")
	t5.SetOutputMirror(writer)
	t5.AppendHeader(table.Row{"Mac Address"})
	t5.AppendRows([]table.Row{
		{metrics.MacAddr},
	})
	t5.SetStyle(table.StyleColoredBright)
	t5.Render()

	fmt.Fprintln(writer)

	// Print Network interfaces and IP addresses
	t6 := table.NewWriter()
	t6.SetTitle("%s", "Network interfaces:")
	t6.SetOutputMirror(writer)
	t6.AppendHeader(table.Row{"Interface", "IP Addresses"})
	for iface, ipaddrs := range metrics.IPAddrs {
		t6.AppendRows([]table.Row{
			{iface, strings.Join(ipaddrs, ", ")},
		})
	}

	t6.SetStyle(table.StyleColoredBright)
	t6.Render()
}
