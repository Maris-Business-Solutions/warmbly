package nodeagent

import (
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

type cpuReading struct {
	scope string
	used  float64
	total float64
	cores float64
	at    time.Time
}

type usageSampler struct {
	previous cpuReading
	readFile func(string) ([]byte, error)
}

func (s *usageSampler) read(path string) string {
	read := s.readFile
	if read == nil {
		read = os.ReadFile
	}
	b, _ := read(path)
	return string(b)
}

func (s *usageSampler) sample(usage *models.NodeUsage) {
	status := keyedNumbers(s.read("/proc/self/status"))
	if rss, ok := status["VmRSS"]; ok {
		mb := int(rss / 1024)
		usage.ResidentMB = &mb
	}
	mem := keyedNumbers(s.read("/proc/meminfo"))
	total, totalOK := mem["MemTotal"]
	available, availableOK := mem["MemAvailable"]
	if totalOK && availableOK && total > 0 && available <= total {
		usedMB, limitMB := int((total-available)/1024), int(total/1024)
		usage.MemoryUsedMB, usage.MemoryLimitMB = &usedMB, &limitMB
		usage.MemoryScope = "host"
	}
	root := s.cgroupPath("")
	used, usedOK := scalar(s.read(filepath.Join(root, "memory.current")))
	limit, limitOK := scalar(s.read(filepath.Join(root, "memory.max")))
	if !usedOK || !limitOK {
		root = s.cgroupPath("memory")
		used, usedOK = scalar(s.read(filepath.Join(root, "memory.usage_in_bytes")))
		limit, limitOK = scalar(s.read(filepath.Join(root, "memory.limit_in_bytes")))
		limitOK = limitOK && limit < 1<<60
	}
	if usedOK && limitOK && used >= 0 && limit > 0 {
		usedMB, limitMB := int(used/1048576), int(limit/1048576)
		usage.MemoryUsedMB, usage.MemoryLimitMB = &usedMB, &limitMB
		usage.MemoryScope = "container"
	}
	current := s.cpu()
	usage.CPUScope = current.scope
	if current.scope != "" && current.scope == s.previous.scope && current.cores == s.previous.cores {
		elapsed, busy := current.total-s.previous.total, current.used-s.previous.used
		if current.scope == "container" {
			elapsed = current.at.Sub(s.previous.at).Seconds() * 1e6 * current.cores
		}
		if elapsed > 0 && busy >= 0 {
			pct := math.Min(100, busy/elapsed*100)
			usage.CPUPercent = &pct
		}
	}
	s.previous = current
}

func (s *usageSampler) cpu() cpuReading {
	root := s.cgroupPath("")
	stat := keyedNumbers(s.read(filepath.Join(root, "cpu.stat")))
	used, ok := stat["usage_usec"]
	cores := float64(runtime.NumCPU())
	bounded := false
	quota := strings.Fields(s.read(filepath.Join(root, "cpu.max")))
	if len(quota) == 2 {
		q, qOK := scalar(quota[0])
		p, pOK := scalar(quota[1])
		if qOK && pOK && p > 0 && q > 0 {
			cores = math.Min(cores, q/p)
			bounded = true
		}
	}
	if !ok {
		used, ok = scalar(s.read(filepath.Join(s.cgroupPath("cpuacct"), "cpuacct.usage")))
		used /= 1000
		q, qOK := scalar(s.read(filepath.Join(s.cgroupPath("cpu"), "cpu.cfs_quota_us")))
		p, pOK := scalar(s.read(filepath.Join(s.cgroupPath("cpu"), "cpu.cfs_period_us")))
		if qOK && pOK && q > 0 && p > 0 {
			cores = math.Min(cores, q/p)
			bounded = true
		}
	}
	cpuset := s.read(filepath.Join(root, "cpuset.cpus.effective"))
	if cpuset == "" {
		cpuset = s.read(filepath.Join(s.cgroupPath("cpuset"), "cpuset.cpus"))
	}
	if cpus := cpuSetSize(cpuset); cpus > 0 {
		cores = math.Min(cores, float64(cpus))
		bounded = bounded || cpus < runtime.NumCPU()
	}
	if ok && cores > 0 && bounded {
		return cpuReading{scope: "container", used: used, cores: cores, at: time.Now()}
	}
	fields := strings.Fields(strings.SplitN(s.read("/proc/stat"), "\n", 2)[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuReading{}
	}
	var total, idle float64
	// Guest time is already included in user/nice; count only the first eight fields.
	for i := 1; i < len(fields) && i <= 8; i++ {
		n, valid := scalar(fields[i])
		if !valid {
			return cpuReading{}
		}
		total += n
		if i == 4 || i == 5 {
			idle += n
		}
	}
	return cpuReading{scope: "host", used: total - idle, total: total}
}

func (s *usageSampler) cgroupPath(controller string) string {
	base := "/sys/fs/cgroup"
	if controller != "" {
		base = filepath.Join(base, controller)
		if (controller == "cpu" || controller == "cpuacct") && s.read(filepath.Join(base, "cgroup.procs")) == "" {
			base = "/sys/fs/cgroup/cpu,cpuacct"
		}
	}
	for _, line := range strings.Split(s.read("/proc/self/cgroup"), "\n") {
		parts := strings.SplitN(line, ":", 3)
		if len(parts) != 3 {
			continue
		}
		for _, name := range strings.Split(parts[1], ",") {
			if name == controller {
				path := filepath.Join(base, filepath.Clean("/"+parts[2]))
				if s.read(filepath.Join(path, "cgroup.procs")) != "" {
					return path
				}
			}
		}
	}
	return base
}

func keyedNumbers(raw string) map[string]float64 {
	out := make(map[string]float64)
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			if n, ok := scalar(fields[1]); ok {
				out[strings.TrimSuffix(fields[0], ":")] = n
			}
		}
	}
	return out
}

func scalar(raw string) (float64, bool) {
	n, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0)
}

func cpuSetSize(raw string) int {
	var count int
	for _, part := range strings.Split(strings.TrimSpace(raw), ",") {
		bounds := strings.SplitN(part, "-", 2)
		low, err := strconv.Atoi(bounds[0])
		if err != nil || low < 0 {
			return 0
		}
		high := low
		if len(bounds) == 2 {
			high, err = strconv.Atoi(bounds[1])
			if err != nil || high < low {
				return 0
			}
		}
		count += high - low + 1
	}
	return count
}
