package mvc

import (
	"bytes"
	"csp2d/ga"
	"csp2d/util"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type Configuration struct {
	Server struct {
		Host      string `yaml:"host"`
		Port      string `yaml:"port"`
		StaticDir string `yaml:"staticDir"`
	} `yaml:"webserver"`
	Defaults struct {
		SimulationParameters SimulationParameters `yaml:"simulationParameters"`
	} `yaml:"defaults"`
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
	var instance *Configuration
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
	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&instance)
	if err != nil {
		util.Error.Fatal(err)
	}
	util.Info.Println("Configuration read")

	return instance
}

func GetConfigurationFromBytes(data []byte) *Configuration {
	var instance *Configuration
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&instance); err != nil {
		util.Error.Fatal(err)
	}
	util.Info.Println("Configuration read from embedded data")
	return instance
}

func GetEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
