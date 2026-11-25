package mvc

import (
	"bytes"
	"csp2d/ga"
	"csp2d/util"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Configuration struct {
	Server struct {
		Host      string `yaml:"host"`
		Port      string `yaml:"port"`
		StaticDir string `yaml:"staticDir"`
		// Pprof serves the net/http/pprof handlers at /debug/pprof/ (diagnostics
		// only; off by default, BE-06).
		Pprof bool `yaml:"pprof"`
	} `yaml:"webserver"`
	Defaults struct {
		// Parameters: each parameter's default value and validation rule
		// (parameters.go, BE-02).
		Parameters ParameterSpecs `yaml:"simulationParameters"`
		// SimulationParameters are the default values of Parameters.
		SimulationParameters SimulationParameters `yaml:"-"`
	} `yaml:"defaults"`
	// Inputs: limits of a start request (Request.Validate).
	Inputs struct {
		MaxPartsCount int `yaml:"maxPartsCount"` // the total part count; the UI checks the same limit (Model.validate)
	} `yaml:"inputs"`
	// Memory: GOMEMLIMIT, GOGC and the arena budget (see memory.go).
	Memory MemoryConfig `yaml:"memory"`
	// Jobs: housekeeping limits; zero or missing values select the defaults
	// (see ga.Limits).
	Jobs struct {
		MaxActiveJobs int           `yaml:"maxActiveJobs"`
		IdleTimeout   time.Duration `yaml:"idleTimeout"`
	} `yaml:"jobs"`
}

// Limits returns the job housekeeping limits.
func (c *Configuration) Limits() ga.Limits {
	return ga.Limits{MaxActiveJobs: c.Jobs.MaxActiveJobs, IdleTimeout: c.Jobs.IdleTimeout}
}

func GetConfiguration(configFile string) *Configuration {
	if configFile == "" {
		configFile = GetEnv("CONFIG_FILE", "application.yaml")
	}

	wd, err := os.Getwd()
	if err != nil {
		util.Error.Fatal(err)
	}
	fmt.Println("Current working directory:", wd)

	f, err := os.Open(configFile)
	if err != nil {
		util.Error.Fatal(err)
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			util.Error.Println(err)
		}
	}(f)
	data, err := io.ReadAll(f)
	if err != nil {
		util.Error.Fatal(err)
	}
	instance, err := ParseConfiguration(data)
	if err != nil {
		util.Error.Fatal(err)
	}
	util.Info.Println("Configuration read")

	return instance
}

func GetConfigurationFromBytes(data []byte) *Configuration {
	instance, err := ParseConfiguration(data)
	if err != nil {
		util.Error.Fatal(err)
	}
	util.Info.Println("Configuration read from embedded data")
	return instance
}

// ParseConfiguration decodes application.yaml, checks the parameter specs and
// the input limits, and fills the default parameter values.
func ParseConfiguration(data []byte) (*Configuration, error) {
	var instance Configuration
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&instance); err != nil {
		return nil, err
	}
	if len(instance.Defaults.Parameters) == 0 {
		return nil, errors.New("defaults.simulationParameters: missing")
	}
	if instance.Inputs.MaxPartsCount < MinPartsCount {
		return nil, fmt.Errorf("inputs.maxPartsCount must be at least %d, got %d", MinPartsCount, instance.Inputs.MaxPartsCount)
	}
	instance.Defaults.SimulationParameters = instance.Defaults.Parameters.Defaults()
	return &instance, nil
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
