package main

import (
	"encoding/json"
	"flag"
	"fmt"
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	lbproxy "github.com/devcyclehq/local-bucketing-proxy"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

var (
	configJSONPath string
	config         lbproxy.ProxyConfig
	printVersion   bool
)

const (
	EnvVarPrefix = "DVC_LB_PROXY"
	Version      = "0.1.0"
)

func init() {

	if os.Getenv(EnvVarPrefix+"_CONFIG") != "" {
		configJSONPath = os.Getenv(EnvVarPrefix + "_PROXY_CONFIG")
	} else {
		flag.StringVar(&configJSONPath, "config", "", "Path to config.json file")
	}

	if os.Getenv(EnvVarPrefix+"_DEBUG") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "DevCycle Local Bucketing Proxy Version %s\n", Version)

		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Printf("\nAlternatively, ")
		_ = envconfig.Usagef(EnvVarPrefix, &lbproxy.ProxyInstance{}, os.Stderr, envconfig.DefaultTableFormat)
	}
	flag.Parse()

	// Load config
	if configJSONPath == "" {
		instance, err := initializeProxyInstanceFromEnv()
		if err != nil {
			log.Println("Failed to initialize proxy instance from environment variables. Please either set the config path or set the environment variables:")
			log.Println(err)
			err = envconfig.Usage(EnvVarPrefix, instance)
			if err != nil {
				log.Println(err)
			}
			return
		}
		defer func() {
			err = os.Remove(instance.UnixSocketPath)
		}()
		config = lbproxy.ProxyConfig{Instances: []lbproxy.ProxyInstance{*instance}}
	} else {
		configFile, err := os.ReadFile(configJSONPath)
		if err != nil {
			log.Println("Failed to read config file, writing an empty configuration file to the specified path.")
			hostname, err := os.Hostname()
			if err != nil {
				hostname = "unknown"
			}
			config := lbproxy.ProxyConfig{Instances: []lbproxy.ProxyInstance{{
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
			}}}
			config.Default()
			bytes, err := json.Marshal(config)
			if err != nil {
				log.Println("Failed to marshal config to JSON.")
				return
			}
			err = os.WriteFile(configJSONPath, bytes, 0644)
			if err != nil {
				log.Println("Failed to write config to file.")
				return
			}
		}

		err = json.Unmarshal(configFile, &config)
		if err != nil {
			return
		}
	}
}

func main() {

	if len(config.Instances) == 0 {
		fmt.Println("No instances found in config.")
		return
	}
	// Create router for each instance
	for _, instance := range config.Instances {
		// Create client
		_, err := lbproxy.NewBucketingProxyInstance(instance)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	// Use a buffered channel, so we don't miss any signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, os.Kill, syscall.SIGTERM)

	// Block until a signal is received.
	s := <-c
	fmt.Println("Got signal:", s)
}

func initializeProxyInstanceFromEnv() (*lbproxy.ProxyInstance, error) {
	instance := &lbproxy.ProxyInstance{}
	err := envconfig.Process(EnvVarPrefix, instance)
	return instance, err
}
