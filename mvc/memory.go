package mvc

import (
	"csp2d/problem/csp2d"
	"csp2d/util"
	"fmt"
	"math"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// MemoryConfig is the `memory` section of application.yaml. Missing values
// select the defaults.
type MemoryConfig struct {
	// Available is the memory the process may use: "auto" (default) = the
	// container (cgroup) limit, else the physical memory; "none" = unknown (no
	// memory limit, unlimited arenas); or a size such as "1GiB", "512MiB" or a
	// number of bytes.
	Available string `yaml:"available"`
	// LimitRatio: GOMEMLIMIT = Available × LimitRatio (default 0.9). The rest is
	// left for memory the Go runtime does not manage (binary, thread stacks of
	// the OS, page cache of the container).
	LimitRatio float64 `yaml:"limitRatio"`
	// GOGC is the GC percent (default 100); -1 turns the proportional GC off
	// and requires a memory limit.
	GOGC *int `yaml:"gogc"`
	// ArenaRatio: all postfix arenas together may hold GOMEMLIMIT × ArenaRatio
	// (default 0.5). The rest of the limit is for ordinary heap objects
	// (populations, forests, arena overflow, HTTP) and the runtime.
	ArenaRatio float64 `yaml:"arenaRatio"`
}

const (
	defaultLimitRatio = 0.9
	defaultArenaRatio = 0.5
	defaultGOGC       = 100
	noMemoryLimit     = int64(math.MaxInt64)
)

// MemoryEnvironment is what ComputeMemorySettings needs from the process and
// the machine; tests replace it.
type MemoryEnvironment struct {
	LookupEnv      func(string) (string, bool)
	CgroupLimit    func() int64 // container limit in bytes, 0 if none
	PhysicalMemory func() int64 // bytes, 0 if unknown
	CurrentLimit   func() int64 // the runtime's memory limit (set from GOMEMLIMIT)
	CurrentGOGC    func() int   // the runtime's GC percent (set from GOGC)
}

// MemorySettings are the effective values, with where each one came from.
type MemorySettings struct {
	Available       int64  // bytes; 0 = unknown
	AvailableSource string // "config", "cgroup", "physical", "none", "unknown"
	MemoryLimit     int64  // bytes; math.MaxInt64 = no limit
	LimitSource     string // "env" (GOMEMLIMIT), "config", "none"
	GCPercent       int    // -1 = off
	GCSource        string // "env" (GOGC), "config"
	ArenaBudget     int64  // bytes; 0 = unlimited
}

func (s MemorySettings) String() string {
	size := func(b int64) string {
		if b <= 0 || b == noMemoryLimit {
			return "none"
		}
		return fmt.Sprintf("%.0f MiB", float64(b)/(1<<20))
	}
	return fmt.Sprintf("memory available %s (%s), GOMEMLIMIT %s (%s), GOGC %d (%s), arena budget %s",
		size(s.Available), s.AvailableSource, size(s.MemoryLimit), s.LimitSource,
		s.GCPercent, s.GCSource, size(s.ArenaBudget))
}

// ParseMemorySize parses "<n>", "<n>B", "<n>KiB", "<n>MiB", "<n>GiB" or
// "<n>TiB" (the GOMEMLIMIT syntax).
func ParseMemorySize(value string) (int64, error) {
	value = strings.TrimSpace(value)
	units := []struct {
		suffix string
		factor int64
	}{{"TiB", 1 << 40}, {"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10}, {"B", 1}}
	factor := int64(1)
	for _, u := range units {
		if strings.HasSuffix(value, u.suffix) {
			value, factor = strings.TrimSuffix(value, u.suffix), u.factor
			break
		}
	}
	n, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || n <= 0 || n > math.MaxInt64/factor {
		return 0, fmt.Errorf("invalid memory size %q (use e.g. 1GiB, 512MiB or a number of bytes)", value)
	}
	return n * factor, nil
}

// ComputeMemorySettings derives GOMEMLIMIT, GOGC and the arena budget.
// GOMEMLIMIT and GOGC set in the environment win over the configuration.
func ComputeMemorySettings(cfg MemoryConfig, env MemoryEnvironment) (MemorySettings, error) {
	var s MemorySettings

	limitRatio, arenaRatio := cfg.LimitRatio, cfg.ArenaRatio
	if limitRatio == 0 {
		limitRatio = defaultLimitRatio
	}
	if arenaRatio == 0 {
		arenaRatio = defaultArenaRatio
	}
	if limitRatio <= 0 || limitRatio > 1 {
		return s, fmt.Errorf("memory.limitRatio %g must be in (0, 1]", limitRatio)
	}
	if arenaRatio <= 0 || arenaRatio >= 1 {
		return s, fmt.Errorf("memory.arenaRatio %g must be in (0, 1)", arenaRatio)
	}

	switch available := strings.TrimSpace(cfg.Available); available {
	case "", "auto":
		cgroup, physical := env.CgroupLimit(), env.PhysicalMemory()
		switch {
		case cgroup > 0 && (physical <= 0 || cgroup <= physical):
			s.Available, s.AvailableSource = cgroup, "cgroup"
		case physical > 0:
			s.Available, s.AvailableSource = physical, "physical"
		default:
			s.AvailableSource = "unknown"
		}
	case "none":
		s.AvailableSource = "none"
	default:
		n, err := ParseMemorySize(available)
		if err != nil {
			return s, fmt.Errorf("memory.available: %w", err)
		}
		s.Available, s.AvailableSource = n, "config"
	}

	if _, set := env.LookupEnv("GOMEMLIMIT"); set {
		s.MemoryLimit, s.LimitSource = env.CurrentLimit(), "env"
	} else if s.Available > 0 {
		s.MemoryLimit, s.LimitSource = int64(float64(s.Available)*limitRatio), "config"
	} else {
		s.MemoryLimit, s.LimitSource = noMemoryLimit, "none"
	}

	if _, set := env.LookupEnv("GOGC"); set {
		s.GCPercent, s.GCSource = env.CurrentGOGC(), "env"
	} else {
		s.GCPercent, s.GCSource = defaultGOGC, "config"
		if cfg.GOGC != nil {
			s.GCPercent = *cfg.GOGC
		}
		if s.GCPercent < 0 && s.MemoryLimit == noMemoryLimit {
			return s, fmt.Errorf("memory.gogc %d (GC off) needs a memory limit, but none is known: set memory.available", s.GCPercent)
		}
	}

	if s.MemoryLimit != noMemoryLimit {
		s.ArenaBudget = int64(float64(s.MemoryLimit) * arenaRatio)
	}
	return s, nil
}

// ApplyMemorySettings computes the settings for this process, applies them to
// the Go runtime and the arena budget, and logs them.
func ApplyMemorySettings(cfg MemoryConfig) (MemorySettings, error) {
	s, err := ComputeMemorySettings(cfg, MemoryEnvironment{
		LookupEnv:      os.LookupEnv,
		CgroupLimit:    func() int64 { return util.CgroupMemoryLimit("/sys/fs/cgroup") },
		PhysicalMemory: util.PhysicalMemory,
		CurrentLimit:   func() int64 { return debug.SetMemoryLimit(-1) },
		CurrentGOGC: func() int {
			current := debug.SetGCPercent(defaultGOGC)
			debug.SetGCPercent(current)
			return current
		},
	})
	if err != nil {
		return s, err
	}
	if s.LimitSource != "env" {
		debug.SetMemoryLimit(s.MemoryLimit)
	}
	if s.GCSource != "env" {
		debug.SetGCPercent(s.GCPercent)
	}
	csp2d.SetArenaBudget(s.ArenaBudget)
	util.Info.Println(s.String())
	if s.MemoryLimit == noMemoryLimit {
		util.Warning.Println("no memory limit is known: arenas are unlimited; set memory.available or GOMEMLIMIT")
	}
	return s, nil
}
