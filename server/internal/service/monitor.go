// Copyright (C) 2026 wolcen@msn.com
// SPDX-License-Identifier: GPL-3.0-or-later
//
// SVNAdmin4 is free software: you can redistribute it and/or modify it under
// the terms of the GNU General Public License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the LICENSE file in the project root for the full license text.

package service

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	colorGreen = "green"
	colorBlue  = "blue"
	colorOrg   = "orange"
	colorRed   = "red"
)

func percentColor(p float64) string {
	switch {
	case p >= 90:
		return colorRed
	case p >= 70:
		return colorOrg
	case p >= 40:
		return colorBlue
	}
	return colorGreen
}

// LoadInfo 系统负载（1/5/15 分钟）
type LoadInfo struct {
	Load1   float64 `json:"load1Min"`
	Load5   float64 `json:"load5Min"`
	Load15  float64 `json:"load15Min"`
	Percent float64 `json:"percent"`
	Color   string  `json:"color"`
	Title   string  `json:"title"`
}

// CPUInfo CPU 使用率与硬件信息
type CPUInfo struct {
	Percent   float64  `json:"percent"`
	Color     string   `json:"color"`
	Models    []string `json:"cpu"`
	Physical  int      `json:"cpuPhysical"`
	Core      int      `json:"cpuCore"`
	Processor int      `json:"cpuProcessor"`
}

// MemInfo 内存使用（MB）
type MemInfo struct {
	Total   float64 `json:"memTotal"`
	Used    float64 `json:"memUsed"`
	Free    float64 `json:"memFree"`
	Percent float64 `json:"percent"`
	Color   string  `json:"color"`
}

// DiskInfo 单个挂载点的磁盘使用
type DiskInfo struct {
	FileSystem string  `json:"fileSystem"`
	MountedOn  string  `json:"mountedOn"`
	Size       float64 `json:"size"`
	Used       float64 `json:"used"`
	Avail      float64 `json:"avail"`
	Percent    float64 `json:"percent"`
	Color      string  `json:"color"`
}

// MonitorInfo 监控总览
type MonitorInfo struct {
	Load LoadInfo   `json:"load"`
	CPU  CPUInfo    `json:"cpu"`
	Mem  MemInfo    `json:"mem"`
	Disk []DiskInfo `json:"disk"`
	Host HostInfo   `json:"host"`
}

// HostInfo 主机基本信息
type HostInfo struct {
	OS        string `json:"os"`
	Platform  string `json:"platform"`
	Arch      string `json:"arch"`
	Hostname  string `json:"hostname"`
	GoVersion string `json:"goVersion"`
	Uptime    int64  `json:"uptime"`
}

// GetMonitorInfo 返回系统监控数据
func GetMonitorInfo() MonitorInfo {
	return MonitorInfo{
		Load: GetLoadInfo(),
		CPU:  GetCPUInfo(),
		Mem:  GetMemInfo(),
		Disk: GetDiskInfo(),
		Host: GetHostInfo(),
	}
}

// GetLoadInfo 读取系统负载
func GetLoadInfo() LoadInfo {
	info := LoadInfo{Title: "系统负载", Color: colorGreen}
	f, err := os.Open("/proc/loadavg")
	if err != nil {
		info.Title = "当前系统不支持读取负载"
		return info
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return info
	}
	fields := strings.Fields(sc.Text())
	parse := func(i int) float64 {
		if len(fields) > i {
			v, err := strconv.ParseFloat(fields[i], 64)
			if err == nil {
				return v
			}
		}
		return 0
	}
	info.Load1 = parse(0)
	info.Load5 = parse(1)
	info.Load15 = parse(2)
	cores := runtime.NumCPU()
	if cores <= 0 {
		cores = 1
	}
	info.Percent = info.Load1 / float64(cores*2) * 100
	if info.Percent > 100 {
		info.Percent = 100
	}
	info.Color = percentColor(info.Percent)
	return info
}

// GetCPUInfo 采样两次 /proc/stat 计算 CPU 使用率
func GetCPUInfo() CPUInfo {
	info := CPUInfo{Processor: runtime.NumCPU(), Color: colorGreen}
	info.Models = cpuModels()
	info.Physical = cpuPhysical()
	info.Core = info.Processor
	s1, t1 := readCPUStat()
	time.Sleep(500 * time.Millisecond)
	s2, t2 := readCPUStat()
	total := t2 - t1
	idle := s2 - s1
	if total > 0 {
		info.Percent = (1 - float64(idle)/float64(total)) * 100
		if info.Percent < 0 {
			info.Percent = 0
		}
		if info.Percent > 100 {
			info.Percent = 100
		}
	}
	info.Color = percentColor(info.Percent)
	return info
}

func readCPUStat() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		var sum uint64
		for i := 1; i < len(fields); i++ {
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				continue
			}
			sum += v
			// 第 4 列是 idle，第 5 列是 iowait
			if i == 4 || i == 5 {
				idle += v
			}
		}
		return idle, sum
	}
	return 0, 0
}

func cpuModels() []string {
	f, err := os.Open("/proc/cpuinfo")
	if err != nil {
		return []string{runtime.GOARCH}
	}
	defer f.Close()
	seen := map[string]bool{}
	var out []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "model name") && !strings.HasPrefix(line, "Hardware") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		name := strings.TrimSpace(line[idx+1:])
		if name != "" && !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return []string{runtime.GOARCH}
	}
	return out
}

func cpuPhysical() int {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return 1
	}
	seen := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "physical id") {
			continue
		}
		idx := strings.Index(line, ":")
		if idx < 0 {
			continue
		}
		seen[strings.TrimSpace(line[idx+1:])] = true
	}
	if len(seen) == 0 {
		return 1
	}
	return len(seen)
}

// GetMemInfo 读取内存使用情况（单位 MB）
func GetMemInfo() MemInfo {
	info := MemInfo{Color: colorGreen}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return info
	}
	defer f.Close()
	vals := map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		key := strings.TrimSuffix(fields[0], ":")
		v, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		vals[key] = v / 1024 // kB -> MB
	}
	total := vals["MemTotal"]
	free := vals["MemFree"]
	// 与原版一致：已用 = 总 - 空闲 - 缓存 - 缓冲 - 可回收
	used := total - free - vals["Cached"] - vals["Buffers"] - vals["SReclaimable"]
	if used < 0 {
		used = 0
	}
	info.Total = round1(total)
	info.Used = round1(used)
	info.Free = round1(total - used)
	if total > 0 {
		info.Percent = round1(used / total * 100)
	}
	info.Color = percentColor(info.Percent)
	return info
}

func round1(v float64) float64 {
	return float64(int(v*10+0.5)) / 10
}

// GetDiskInfo 读取磁盘挂载点使用情况（单位 GB）
func GetDiskInfo() []DiskInfo {
	seen := map[string]bool{}
	var out []DiskInfo
	for _, m := range readMounts() {
		if !strings.HasPrefix(m.mountPoint, "/") || seen[m.mountPoint] {
			continue
		}
		st, ok := statFS(m.mountPoint)
		if !ok || st.total == 0 {
			continue
		}
		seen[m.mountPoint] = true
		used := st.total - st.avail
		p := float64(used) / float64(st.total) * 100
		out = append(out, DiskInfo{
			FileSystem: m.fsType,
			MountedOn:  m.mountPoint,
			Size:       round1(float64(st.total) / (1 << 30)),
			Used:       round1(float64(used) / (1 << 30)),
			Avail:      round1(float64(st.avail) / (1 << 30)),
			Percent:    round1(p),
			Color:      percentColor(p),
		})
	}
	return out
}

type mountEntry struct {
	fsType     string
	mountPoint string
}

func readMounts() []mountEntry {
	var out []mountEntry
	for _, path := range []string{"/proc/mounts", "/etc/mtab"} {
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 2 {
				continue
			}
			out = append(out, mountEntry{fsType: fields[0], mountPoint: fields[1]})
		}
		f.Close()
		if len(out) > 0 {
			break
		}
	}
	return out
}

// GetHostInfo 主机信息
func GetHostInfo() HostInfo {
	info := HostInfo{
		Platform:  runtime.GOOS,
		Arch:      runtime.GOARCH,
		GoVersion: runtime.Version(),
		OS:        osName(),
	}
	if h, err := os.Hostname(); err == nil {
		info.Hostname = h
	}
	info.Uptime = uptimeSeconds()
	return info
}

func osName() string {
	candidates := []string{
		"/etc/redhat-release",
		"/etc/lsb-release",
		"/etc/debian_version",
		"/etc/fedora-release",
		"/etc/SuSE-release",
		"/etc/arch-release",
		"/etc/os-release",
	}
	for _, p := range candidates {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if strings.HasPrefix(line, "PRETTY_NAME=") || strings.HasPrefix(line, "NAME=") {
				return strings.Trim(strings.TrimPrefix(strings.TrimPrefix(line, "PRETTY_NAME="), "NAME="), `"`)
			}
			if p == "/etc/debian_version" {
				return "Debian " + line
			}
			return line
		}
	}
	return runtime.GOOS
}

func uptimeSeconds() int64 {
	data, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0
	}
	return int64(v)
}
