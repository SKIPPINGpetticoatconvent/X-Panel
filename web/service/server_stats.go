package service

import (
	"fmt"
	"runtime"
	"time"

	"x-ui/logger"
	"x-ui/util/sys"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/load"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/net"
)

// 系统统计监控模块
// 负责系统资源监控、性能统计、应用程序状态跟踪等功能

func (s *ServerService) GetSystemStatus(lastStatus *Status) *Status {
	now := time.Now()
	status := &Status{
		T: now,
	}

	// CPU stats
	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
	} else {
		status.Cpu = percents[0]
	}

	status.CpuCores, err = cpu.Counts(false)
	if err != nil {
		logger.Warning("get cpu cores count failed:", err)
	}

	status.LogicalPro = runtime.NumCPU()

	cpuInfos, err := cpu.Info()
	if err != nil {
		logger.Warning("get cpu info failed:", err)
	} else if len(cpuInfos) > 0 {
		status.CpuSpeedMhz = cpuInfos[0].Mhz
	} else {
		logger.Warning("could not find cpu info")
	}

	// Uptime
	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
	} else {
		status.Uptime = upTime
	}

	// Memory stats
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("get swap memory failed:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	// Disk stats
	diskInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("get disk usage failed:", err)
	} else {
		status.Disk.Current = diskInfo.Used
		status.Disk.Total = diskInfo.Total
	}

	// Load averages
	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed:", err)
	} else {
		status.Loads = []float64{avgState.Load1, avgState.Load5, avgState.Load15}
	}

	// Network stats
	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		if lastStatus != nil {
			duration := now.Sub(lastStatus.T)
			seconds := float64(duration) / float64(time.Second)
			up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
			down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
			status.NetIO.Up = up
			status.NetIO.Down = down
		}
	} else {
		logger.Warning("can not find io counters")
	}

	// TCP/UDP connections
	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("get udp connections failed:", err)
	}

	// IP fetching with caching
	showIp4ServiceLists := []string{
		"https://api4.ipify.org",
		"https://ipv4.icanhazip.com",
		"https://v4.api.ipinfo.io/ip",
		"https://ipv4.myexternalip.com/raw",
		"https://4.ident.me",
		"https://check-host.net/ip",
	}
	showIp6ServiceLists := []string{
		"https://api6.ipify.org",
		"https://ipv6.icanhazip.com",
		"https://v6.api.ipinfo.io/ip",
		"https://ipv6.myexternalip.com/raw",
		"https://6.ident.me",
	}

	if s.cachedIPv4 == "" {
		for _, ip4Service := range showIp4ServiceLists {
			s.cachedIPv4 = getPublicIP(ip4Service)
			if s.cachedIPv4 != "N/A" {
				break
			}
		}
	}

	if s.cachedIPv6 == "" && !s.noIPv6 {
		for _, ip6Service := range showIp6ServiceLists {
			s.cachedIPv6 = getPublicIP(ip6Service)
			if s.cachedIPv6 != "N/A" {
				break
			}
		}
	}

	if s.cachedIPv6 == "N/A" {
		s.noIPv6 = true
	}

	status.PublicIP.IPv4 = s.cachedIPv4
	status.PublicIP.IPv6 = s.cachedIPv6

	// Xray status
	if s.xrayService.IsXrayRunning() {
		status.Xray.State = Running
		status.Xray.ErrorMsg = ""
	} else {
		err := s.xrayService.GetXrayErr()
		if err != nil {
			status.Xray.State = Error
		} else {
			status.Xray.State = Stop
		}
		status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
	}
	status.Xray.Version = s.xrayService.GetXrayVersion()

	// Application stats
	var rtm runtime.MemStats
	runtime.ReadMemStats(&rtm)
	status.AppStats.Mem = rtm.Sys
	status.AppStats.Threads = uint32(runtime.NumGoroutine())
	if p != nil && p.IsRunning() {
		status.AppStats.Uptime = p.GetUptime()
	} else {
		status.AppStats.Uptime = 0
	}

	return status
}

// GetCPUInfo 获取CPU详细信息
func (s *ServerService) GetCPUInfo() (map[string]interface{}, error) {
	cpuInfos, err := cpu.Info()
	if err != nil {
		return nil, err
	}

	if len(cpuInfos) == 0 {
		return nil, fmt.Errorf("no CPU info found")
	}

	info := cpuInfos[0]
	return map[string]interface{}{
		"ModelName":  info.ModelName,
		"Mhz":        info.Mhz,
		"Cores":      info.Cores,
		"PhysicalId": info.PhysicalID,
		"Family":     info.Family,
		"Stepping":   info.Stepping,
	}, nil
}

// GetMemInfo 获取内存详细信息
func (s *ServerService) GetMemInfo() (map[string]interface{}, error) {
	memInfo, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"Total":       memInfo.Total,
		"Available":   memInfo.Available,
		"Used":        memInfo.Used,
		"UsedPercent": memInfo.UsedPercent,
		"Free":        memInfo.Free,
		"Active":      memInfo.Active,
		"Inactive":    memInfo.Inactive,
		"Wired":       memInfo.Wired,
		"Buffers":     memInfo.Buffers,
		"Cached":      memInfo.Cached,
	}, nil
}

// GetDiskInfo 获取磁盘详细信息
func (s *ServerService) GetDiskInfo() (map[string]interface{}, error) {
	diskInfo, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"Total":        diskInfo.Total,
		"Free":         diskInfo.Free,
		"Used":         diskInfo.Used,
		"UsedPercent":  diskInfo.UsedPercent,
		"InodesTotal":  diskInfo.InodesTotal,
		"InodesFree":   diskInfo.InodesFree,
		"InodesUsed":   diskInfo.InodesUsed,
		"Fstype":       diskInfo.Fstype,
	}, nil
}

// GetNetworkInfo 获取网络详细信息
func (s *ServerService) GetNetworkInfo() (map[string]interface{}, error) {
	ioStats, err := net.IOCounters(false)
	if err != nil {
		return nil, err
	}

	if len(ioStats) == 0 {
		return nil, fmt.Errorf("no network info found")
	}

	stat := ioStats[0]
	return map[string]interface{}{
		"BytesSent":   stat.BytesSent,
		"BytesRecv":   stat.BytesRecv,
		"PacketsSent": stat.PacketsSent,
		"PacketsRecv": stat.PacketsRecv,
		"Errin":       stat.Errin,
		"Errout":      stat.Errout,
		"Dropin":      stat.Dropin,
		"Dropout":     stat.Dropout,
		"Fifoin":      stat.Fifoin,
		"Fifoout":     stat.Fifoout,
	}, nil
}