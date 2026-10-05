package nodeagent

import (
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

func samplerWithFiles(files map[string]string) *usageSampler {
	return &usageSampler{readFile: func(path string) ([]byte, error) {
		if value, ok := files[path]; ok {
			return []byte(value), nil
		}
		return nil, os.ErrNotExist
	}}
}

func TestHostUsageSeparatesResidentMemoryAndMachineMemory(t *testing.T) {
	files := map[string]string{
		"/proc/self/status": "VmRSS: 140288 kB\n",
		"/proc/meminfo":     "MemTotal: 8388608 kB\nMemAvailable: 6291456 kB\n",
		"/proc/stat":        "cpu 100 0 100 800 0 0 0 0 50 0\n",
	}
	s := samplerWithFiles(files)
	var first models.NodeUsage
	s.sample(&first)
	if first.CPUPercent != nil || *first.ResidentMB != 137 || *first.MemoryUsedMB != 2048 || *first.MemoryLimitMB != 8192 || first.MemoryScope != "host" {
		t.Fatalf("unexpected initial snapshot: %+v", first)
	}
	files["/proc/stat"] = "cpu 125 0 125 850 0 0 0 0 100 0\n"
	var next models.NodeUsage
	s.sample(&next)
	if next.CPUScope != "host" || next.CPUPercent == nil || *next.CPUPercent != 50 {
		t.Fatalf("expected 50%% host CPU excluding guest double counting: %+v", next)
	}
}

func TestContainerUsageUsesLimitsAndCPUInterval(t *testing.T) {
	files := map[string]string{
		"/proc/meminfo":                 "MemTotal: 8388608 kB\nMemAvailable: 6291456 kB\n",
		"/sys/fs/cgroup/memory.current": "143654912",
		"/sys/fs/cgroup/memory.max":     "536870912",
		"/sys/fs/cgroup/cpu.stat":       "usage_usec 1250000\n",
		"/sys/fs/cgroup/cpu.max":        "50000 100000",
	}
	s := samplerWithFiles(files)
	s.previous = cpuReading{scope: "container", used: 1000000, cores: 0.5, at: time.Now().Add(-time.Second)}
	var usage models.NodeUsage
	s.sample(&usage)
	if usage.MemoryScope != "container" || *usage.MemoryUsedMB != 137 || *usage.MemoryLimitMB != 512 || usage.CPUPercent == nil || *usage.CPUPercent < 45 || *usage.CPUPercent > 51 {
		t.Fatalf("expected container 137/512 MiB, approximately 50%% CPU: %+v", usage)
	}
	files["/sys/fs/cgroup/cpu.max"] = "25000 100000"
	var changed models.NodeUsage
	s.sample(&changed)
	if changed.CPUPercent != nil {
		t.Fatal("a changed CPU quota must establish a new baseline")
	}
}

func TestMissingTelemetryIsNotZero(t *testing.T) {
	s := samplerWithFiles(nil)
	var usage models.NodeUsage
	s.sample(&usage)
	if usage.CPUPercent != nil || usage.ResidentMB != nil || usage.MemoryUsedMB != nil || usage.MemoryLimitMB != nil {
		t.Fatalf("missing OS metrics must remain unavailable: %+v", usage)
	}
}

func TestCgroupV1AndNestedV2Memory(t *testing.T) {
	for _, version := range []string{"v1", "v2"} {
		t.Run(version, func(t *testing.T) {
			files := map[string]string{"/proc/meminfo": "MemTotal: 8388608 kB\nMemAvailable: 6291456 kB\n"}
			if version == "v1" {
				files["/sys/fs/cgroup/memory/memory.usage_in_bytes"] = "143654912"
				files["/sys/fs/cgroup/memory/memory.limit_in_bytes"] = "536870912"
			} else {
				files["/proc/self/cgroup"] = "0::/worker\n"
				files["/sys/fs/cgroup/worker/cgroup.procs"] = "1\n"
				files["/sys/fs/cgroup/worker/memory.current"] = "143654912"
				files["/sys/fs/cgroup/worker/memory.max"] = "536870912"
			}
			var usage models.NodeUsage
			samplerWithFiles(files).sample(&usage)
			if usage.MemoryScope != "container" || *usage.MemoryLimitMB != 512 {
				t.Fatalf("unexpected %s memory snapshot: %+v", version, usage)
			}
		})
	}
}

func TestCPUSetSize(t *testing.T) {
	for raw, want := range map[string]int{"0-3,6,8-9": 7, "0": 1, "": 0, "3-1": 0, "no": 0} {
		if got := cpuSetSize(raw); got != want {
			t.Errorf("cpuSetSize(%q) = %d, want %d", raw, got, want)
		}
	}
}

func TestContainerMemoryDoesNotRequireHostMemory(t *testing.T) {
	files := map[string]string{
		"/sys/fs/cgroup/memory.current": "143654912",
		"/sys/fs/cgroup/memory.max":     "536870912",
	}
	var usage models.NodeUsage
	samplerWithFiles(files).sample(&usage)
	if usage.MemoryScope != "container" || usage.MemoryUsedMB == nil || *usage.MemoryUsedMB != 137 || *usage.MemoryLimitMB != 512 {
		t.Fatalf("expected cgroup memory even without host totals: %+v", usage)
	}
}

func TestContainerCPUQuotaEqualToAvailableCPUs(t *testing.T) {
	files := map[string]string{
		"/sys/fs/cgroup/cpu.stat": "usage_usec 1250000\n",
		"/sys/fs/cgroup/cpu.max":  strconv.Itoa(runtime.NumCPU()*100000) + " 100000",
	}
	var usage models.NodeUsage
	samplerWithFiles(files).sample(&usage)
	if usage.CPUScope != "container" {
		t.Fatalf("a finite CPU quota should retain container scope: %+v", usage)
	}
}
