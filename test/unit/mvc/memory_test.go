//go:build testhooks

package mvc_test

import (
	"csp2d/mvc"
	"math"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// Variables, not constants: float arithmetic on them must happen at run time,
// exactly as ComputeMemorySettings does it.
var (
	mib = int64(1) << 20
	gib = int64(1) << 30
)

func fakeEnv(env map[string]string, cgroup, physical int64) mvc.MemoryEnvironment {
	return mvc.MemoryEnvironment{
		LookupEnv:      func(k string) (string, bool) { v, ok := env[k]; return v, ok },
		CgroupLimit:    func() int64 { return cgroup },
		PhysicalMemory: func() int64 { return physical },
		CurrentLimit:   func() int64 { return 300 * mib }, // what the runtime parsed from GOMEMLIMIT
		CurrentGOGC:    func() int { return 50 },          // what the runtime parsed from GOGC
	}
}

func intPtr(v int) *int { return &v }

// UT-MEM-01: GOMEMLIMIT and the arena budget follow the container limit.
func TestMemorySettingsFromContainerLimit(t *testing.T) {
	s, err := mvc.ComputeMemorySettings(mvc.MemoryConfig{}, fakeEnv(nil, 2*gib, 16*gib))
	if err != nil {
		t.Fatal(err)
	}
	if s.Available != 2*gib || s.AvailableSource != "cgroup" {
		t.Errorf("available %d (%s), want 2 GiB from the cgroup", s.Available, s.AvailableSource)
	}
	if want := int64(float64(2*gib) * 0.9); s.MemoryLimit != want || s.LimitSource != "config" {
		t.Errorf("limit %d (%s), want %d", s.MemoryLimit, s.LimitSource, want)
	}
	if want := int64(float64(s.MemoryLimit) * 0.5); s.ArenaBudget != want {
		t.Errorf("arena budget %d, want %d", s.ArenaBudget, want)
	}
	if s.GCPercent != 100 || s.GCSource != "config" {
		t.Errorf("GOGC %d (%s), want 100 from the config default", s.GCPercent, s.GCSource)
	}
}

// UT-MEM-02: without a container limit the physical memory is used; with
// neither, there is no memory limit and the arenas are unlimited.
func TestMemorySettingsWithoutContainer(t *testing.T) {
	s, err := mvc.ComputeMemorySettings(mvc.MemoryConfig{}, fakeEnv(nil, 0, 8*gib))
	if err != nil {
		t.Fatal(err)
	}
	if s.Available != 8*gib || s.AvailableSource != "physical" {
		t.Errorf("available %d (%s), want 8 GiB physical", s.Available, s.AvailableSource)
	}
	// A cgroup limit larger than the machine is not a real limit.
	s, _ = mvc.ComputeMemorySettings(mvc.MemoryConfig{}, fakeEnv(nil, 64*gib, 8*gib))
	if s.Available != 8*gib {
		t.Errorf("available %d, want the smaller physical memory", s.Available)
	}
	s, err = mvc.ComputeMemorySettings(mvc.MemoryConfig{}, fakeEnv(nil, 0, 0))
	if err != nil {
		t.Fatal(err)
	}
	if s.MemoryLimit != math.MaxInt64 || s.ArenaBudget != 0 || s.AvailableSource != "unknown" {
		t.Errorf("unknown memory: limit %d budget %d source %s", s.MemoryLimit, s.ArenaBudget, s.AvailableSource)
	}
}

// UT-MEM-03: explicit configuration, and GOMEMLIMIT / GOGC from the
// environment win over it.
func TestMemorySettingsConfigAndEnv(t *testing.T) {
	cfg := mvc.MemoryConfig{Available: "1GiB", LimitRatio: 0.8, GOGC: intPtr(200), ArenaRatio: 0.25}
	s, err := mvc.ComputeMemorySettings(cfg, fakeEnv(nil, 4*gib, 16*gib))
	if err != nil {
		t.Fatal(err)
	}
	if s.Available != gib || s.AvailableSource != "config" || s.MemoryLimit != int64(float64(gib)*0.8) ||
		s.GCPercent != 200 || s.ArenaBudget != int64(float64(s.MemoryLimit)*0.25) {
		t.Errorf("explicit config not applied: %+v", s)
	}
	s, err = mvc.ComputeMemorySettings(cfg, fakeEnv(map[string]string{"GOMEMLIMIT": "300MiB", "GOGC": "50"}, 4*gib, 16*gib))
	if err != nil {
		t.Fatal(err)
	}
	if s.MemoryLimit != 300*mib || s.LimitSource != "env" || s.GCPercent != 50 || s.GCSource != "env" {
		t.Errorf("environment must win: %+v", s)
	}
	if s.ArenaBudget != int64(float64(300*mib)*0.25) {
		t.Errorf("arena budget %d must follow the effective limit", s.ArenaBudget)
	}
}

// UT-MEM-04: invalid settings are rejected; GC off needs a memory limit.
func TestMemorySettingsValidation(t *testing.T) {
	bad := map[string]mvc.MemoryConfig{
		"size":        {Available: "1GB"},
		"limit ratio": {LimitRatio: 1.5},
		"arena ratio": {ArenaRatio: 1},
		"gc off":      {Available: "none", GOGC: intPtr(-1)},
	}
	for name, cfg := range bad {
		if _, err := mvc.ComputeMemorySettings(cfg, fakeEnv(nil, 0, 0)); err == nil {
			t.Errorf("%s: accepted %+v", name, cfg)
		}
	}
	if s, err := mvc.ComputeMemorySettings(mvc.MemoryConfig{Available: "512MiB", GOGC: intPtr(-1)}, fakeEnv(nil, 0, 0)); err != nil || s.GCPercent != -1 {
		t.Errorf("GC off with a limit: %+v %v", s, err)
	}
}

// UT-MEM-05: the size syntax is GOMEMLIMIT's (B, KiB, MiB, GiB, TiB).
func TestParseMemorySize(t *testing.T) {
	ok := map[string]int64{"1073741824": gib, "512MiB": 512 * mib, "2GiB": 2 * gib, "64KiB": 64 << 10, "100B": 100, "1TiB": 1 << 40}
	for in, want := range ok {
		if got, err := mvc.ParseMemorySize(in); err != nil || got != want {
			t.Errorf("%q: %d %v, want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "1GB", "-1MiB", "0", "1.5GiB", "abc"} {
		if _, err := mvc.ParseMemorySize(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

// UT-MEM-06: the memory section decodes from YAML, including a plain number
// of bytes for available.
func TestMemoryConfigYAML(t *testing.T) {
	var c mvc.Configuration
	src := "memory:\n  available: 2147483648\n  limitRatio: 0.85\n  gogc: -1\n  arenaRatio: 0.4\n"
	if err := yaml.Unmarshal([]byte(src), &c); err != nil {
		t.Fatal(err)
	}
	m := c.Memory
	if m.Available != "2147483648" || m.LimitRatio != 0.85 || m.GOGC == nil || *m.GOGC != -1 || m.ArenaRatio != 0.4 {
		t.Fatalf("decoded %+v", m)
	}
	if !strings.Contains(mvc.MemorySettings{}.String(), "GOMEMLIMIT") {
		t.Error("settings summary must name GOMEMLIMIT")
	}
}
