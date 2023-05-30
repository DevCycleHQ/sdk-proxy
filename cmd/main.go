package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	lbproxy "github.com/devcyclehq/local-bucketing-proxy"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
)

const (
	EnvVarPrefix = "DVC_LB_PROXY"
	Version      = "0.1.0"

	EnvConfigFormat = `
This application can also be configured via the environment. The following environment
variables can be used:

{{printf "%-54s" "KEY"}}	{{ printf "%-11s" "TYPE" }}	DEFAULT	 REQUIRED	DESCRIPTION
{{range .}}{{usage_key . | printf "%-54s"}}	{{usage_type . | printf "%-11s"}}	{{usage_default .}}	{{usage_required . | printf "%5s" }}	{{usage_description .}}
{{end}}`
)

// For parsing just the config filename, before we know the intended config mechanism
type InitialConfig struct {
	ConfigPath string `envconfig:"CONFIG" desc:"The path to a JSON config file."`
	Debug      bool   `envconfig:"DEBUG" default:"false" desc:"Whether to enable debug mode."`
}

// For parsing the full config along with the proxy settings
type FullEnvConfig struct {
	InitialConfig
	lbproxy.ProxyInstance
}

// TODO: this is complicated enough that we need tests for it
func parseConfig() (*lbproxy.ProxyConfig, error) {
	var initialConfig InitialConfig
	var proxyConfig lbproxy.ProxyConfig

	flag.StringVar(&initialConfig.ConfigPath, "config", "", "The path to a JSON config file.")

	flag.Usage = func() {
		log.Printf("DevCycle Local Bucketing Proxy Version %s\n", Version)

		log.Printf("Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		_ = envconfig.Usagef(EnvVarPrefix, &FullEnvConfig{}, os.Stderr, EnvConfigFormat)
	}
	flag.Parse()

	// Load config from environment variables
	if initialConfig.ConfigPath == "" {
		var fullEnvConfig FullEnvConfig
		log.Println("No config path provided, reading configuration from environment variables.")
		err := envconfig.Process(EnvVarPrefix, &fullEnvConfig)

		if err != nil {
			return nil, err
		}
		proxyConfig.Instances = append(proxyConfig.Instances, &fullEnvConfig.ProxyInstance)
	} else {
		// Load config from JSON file
		configData, err := os.ReadFile(initialConfig.ConfigPath)
		if err != nil {
			log.Printf("Failed to read config file, writing a default configuration file to the specified path: %s", initialConfig.ConfigPath)
			proxyConfig = sampleProxyConfig()

			sampleConfigData, err := json.MarshalIndent(proxyConfig, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to marshal sample config to JSON: %w", err)
			}
			err = os.WriteFile(initialConfig.ConfigPath, sampleConfigData, 0644)
			if err != nil {
				return nil, fmt.Errorf("Failed to write sample config to file: %w", err)
			}
			log.Fatal("Add your SDK key to the config file and run this command again.")
		}

		err = json.Unmarshal(configData, &proxyConfig)
		if err != nil {
			return nil, err
		}
	}

	if !initialConfig.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	return &proxyConfig, nil
}

func main() {
	config, err := parseConfig()
	if err != nil {
		log.Printf("Failed to parse config: %s", err)
		log.Fatal("Please either set the config path or set the environment variables")
	}

	if len(config.Instances) == 0 {
		log.Fatalf("No instances found in config. Use %s -config <path> to create a sample config file.", os.Args[0])
		return
	}
	// Create router for each instance
	for _, instance := range config.Instances {
		log.Printf("Creating bucketing proxy instance: %+v", instance)

		// Create client
		_, err := lbproxy.NewBucketingProxyInstance(instance)
		if err != nil {
			log.Fatal(err)
		}
		defer func(path string) {
			err = os.Remove(path)
		}(instance.UnixSocketPath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Use a buffered channel, so we don't miss any signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Block until a signal is received.
		s := <-c
		fmt.Printf("Received signal: %s, shutting down", s)

		for _, instance := range config.Instances {
			err := instance.Close()
			if err != nil {
				log.Printf("Failed to shut down instance: %s", err)
			}
		}

		cancel()
	}()

	<-ctx.Done()
}

func sampleProxyConfig() lbproxy.ProxyConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	proxyConfig := lbproxy.ProxyConfig{
		Instances: []*lbproxy.ProxyInstance{{
			UnixSocketPath:    "/tmp/devcycle.sock",
			HTTPPort:          8080,
			UnixSocketEnabled: false,
			HTTPEnabled:       true,
			SDKKey:            "",
			PlatformData: devcycle.PlatformData{
				SdkType:         "server",
				SdkVersion:      devcycle.VERSION,
				PlatformVersion: runtime.Version(),
				Platform:        "Go",
				Hostname:        hostname,
			},
			SDKConfig: lbproxy.SDKConfig{
				EventFlushIntervalMS:         0,
				ConfigPollingIntervalMS:      0,
				RequestTimeout:               0,
				DisableAutomaticEventLogging: false,
				DisableCustomEventLogging:    false,
				MaxEventQueueSize:            0,
				FlushEventQueueSize:          0,
				ConfigCDNURI:                 "",
				EventsAPIURI:                 "",
			},
		}},
	}
	proxyConfig.Default()
	return proxyConfig
}
