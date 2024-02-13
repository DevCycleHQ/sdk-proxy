package local_bucketing_proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
	"github.com/kelseyhightower/envconfig"
)

const (
	EnvVarPrefix = "DVC_LB_PROXY"
)

type ProxyConfig struct {
	Instances []*ProxyInstance `json:"instances"`
}

type ProxyInstance struct {
	UnixSocketPath        string                `json:"unixSocketPath" envconfig:"UNIX_SOCKET_PATH" desc:"The path to the Unix socket."`
	UnixSocketPermissions int                   `json:"unixSocketPermissions" envconfig:"UNIX_SOCKET_PERMISSIONS" default:"755" desc:"The permissions to set on the Unix socket. Defaults to 0755"`
	UnixSocketEnabled     bool                  `json:"unixSocketEnabled" envconfig:"UNIX_SOCKET_ENABLED" default:"false" desc:"Whether to enable the Unix socket. Defaults to false."`
	HTTPPort              int                   `json:"httpPort" envconfig:"HTTP_PORT" default:"8080" desc:"The port to listen on for HTTP requests. Defaults to 8080."`
	HTTPEnabled           bool                  `json:"httpEnabled" envconfig:"HTTP_ENABLED" default:"true" desc:"Whether to enable the HTTP server. Defaults to true."`
	SDKKey                string                `json:"sdkKey" required:"true" envconfig:"SDK_KEY" desc:"The Server SDK key to use for this instance."`
	LogFile               string                `json:"logFile" default:"/var/log/devcycle.log" envconfig:"LOG_FILE" desc:"The path to the log file. Defaults to /var/log/devcycle.log"`
	PlatformData          devcycle.PlatformData `json:"platformData" required:"true"`
	SDKConfig             SDKConfig             `json:"sdkConfig" required:"true"`
	dvcClient             *devcycle.Client
}

type SDKConfig struct {
	EventFlushIntervalMS         int64  `json:"eventFlushIntervalMS,omitempty" split_words:"true" desc:"The interval at which events are flushed to the events api in milliseconds."`
	ConfigPollingIntervalMS      int64  `json:"configPollingIntervalMS,omitempty" split_words:"true" desc:"The interval at which the SDK polls the config CDN for updates in milliseconds."`
	RequestTimeout               int64  `json:"requestTimeout,omitempty" split_words:"true" desc:"The timeout for requests to the config CDN and events API in milliseconds."`
	DisableAutomaticEventLogging bool   `json:"disableAutomaticEventLogging,omitempty" split_words:"true" default:"false" desc:"Whether to disable automatic event logging. Defaults to false."`
	DisableCustomEventLogging    bool   `json:"disableCustomEventLogging,omitempty" split_words:"true" default:"false" desc:"Whether to disable custom event logging. Defaults to false."`
	MaxEventQueueSize            int    `json:"maxEventsPerFlush,omitempty" split_words:"true" desc:"The maximum number of events to be in the queue before dropping events."`
	FlushEventQueueSize          int    `json:"minEventsPerFlush,omitempty" split_words:"true" desc:"The minimum number of events to be in the queue before flushing events."`
	ConfigCDNURI                 string `json:"configCDNURI,omitempty" envconfig:"CONFIG_CDN_URI" desc:"The URI of the Config CDN - leave unspecified if not needing an outbound proxy."`
	EventsAPIURI                 string `json:"eventsAPIURI,omitempty" envconfig:"EVENTS_API_URI" desc:"The URI of the Events API - leave unspecified if not needing an outbound proxy."`
}

func (i *ProxyInstance) Close() error {
	return i.dvcClient.Close()
}

func (i *ProxyInstance) BuildDevCycleOptions() *devcycle.Options {
	options := devcycle.Options{
		EnableEdgeDB:                 false,
		EnableCloudBucketing:         false,
		EventFlushIntervalMS:         time.Duration(i.SDKConfig.EventFlushIntervalMS) * time.Millisecond,
		ConfigPollingIntervalMS:      time.Duration(i.SDKConfig.ConfigPollingIntervalMS) * time.Millisecond,
		RequestTimeout:               time.Duration(i.SDKConfig.RequestTimeout) * time.Millisecond,
		DisableAutomaticEventLogging: i.SDKConfig.DisableAutomaticEventLogging,
		DisableCustomEventLogging:    i.SDKConfig.DisableCustomEventLogging,
		MaxEventQueueSize:            i.SDKConfig.MaxEventQueueSize,
		FlushEventQueueSize:          i.SDKConfig.FlushEventQueueSize,
		ConfigCDNURI:                 i.SDKConfig.ConfigCDNURI,
		EventsAPIURI:                 i.SDKConfig.EventsAPIURI,
		Logger:                       nil,
		UseDebugWASM:                 false,
		AdvancedOptions: devcycle.AdvancedOptions{
			OverridePlatformData: &i.PlatformData,
		},
	}
	options.CheckDefaults()
	return &options
}

func (i *ProxyInstance) Default() {
	i.SDKConfig.Default()
	if i.HTTPEnabled && i.HTTPPort == 0 {
		i.HTTPPort = 8080
	}
	if i.LogFile == "" {
		i.LogFile = "/var/log/devcycle.log"
	}
	if i.UnixSocketEnabled {
		if i.UnixSocketPath == "" {
			i.UnixSocketPath = "/tmp/devcycle.sock"
		}
		if i.UnixSocketPermissions == 0 {
			i.UnixSocketPermissions = 755
		}
	}
}
func (c *ProxyConfig) Default() {
	for i := range c.Instances {
		c.Instances[i].Default()
	}
}

func (c *SDKConfig) Default() {
	if c.EventFlushIntervalMS == 0 {
		c.EventFlushIntervalMS = 3000
	}
	if c.ConfigPollingIntervalMS == 0 {
		c.ConfigPollingIntervalMS = 30000
	}
	if c.RequestTimeout == 0 {
		c.RequestTimeout = 30000
	}
	if c.MaxEventQueueSize == 0 {
		c.MaxEventQueueSize = 10000
	}
	if c.FlushEventQueueSize == 0 {
		c.FlushEventQueueSize = 100
	}
	if c.ConfigCDNURI == "" {
		c.ConfigCDNURI = "https://config-cdn.devcycle.com"
	}
	if c.EventsAPIURI == "" {
		c.EventsAPIURI = "https://events.devcycle.com"
	}
}

// For parsing just the config filename, before we know the intended config mechanism
type InitialConfig struct {
	ConfigPath string `envconfig:"CONFIG" desc:"The path to a JSON config file."`
	Debug      bool   `envconfig:"DEBUG" default:"false" desc:"Whether to enable debug mode."`
}

// For parsing the full config along with the proxy settings
type FullEnvConfig struct {
	InitialConfig
	ProxyInstance
}

// Parse the config from either a JSON file or environment variables
func ParseConfig(configPath string) (*ProxyConfig, error) {
	var proxyConfig ProxyConfig
	initialConfig := InitialConfig{
		ConfigPath: configPath,
	}

	// Attempt to load initial config from environment, ignoring any errors
	_ = envconfig.Process(EnvVarPrefix, &initialConfig)

	// Load full config from environment variables
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
		log.Printf("Loading configuration from file: %s", initialConfig.ConfigPath)
		configData, err := os.ReadFile(initialConfig.ConfigPath)
		if err != nil {
			log.Printf("Failed to read config file, writing a default configuration file to the specified path: %s", initialConfig.ConfigPath)
			proxyConfig = SampleProxyConfig()

			sampleConfigData, err := json.MarshalIndent(proxyConfig, "", "  ")
			if err != nil {
				return nil, fmt.Errorf("failed to write sample config to JSON: %w", err)
			}
			err = os.WriteFile(initialConfig.ConfigPath, sampleConfigData, 0644)
			if err != nil {
				return nil, fmt.Errorf("Failed to write sample config to file: %w", err)
			}
			log.Fatal("Add your SDK key to the config file and run this command again.")
		}

		err = json.Unmarshal(configData, &proxyConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to parse config from JSON: %w", err)
		}
		proxyConfig.Default()
	}

	if !initialConfig.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	return &proxyConfig, nil
}

func SampleProxyConfig() ProxyConfig {
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	proxyConfig := ProxyConfig{
		Instances: []*ProxyInstance{{
			UnixSocketPath:        "/tmp/devcycle.sock",
			HTTPPort:              8080,
			UnixSocketEnabled:     false,
			UnixSocketPermissions: 755,
			HTTPEnabled:           true,
			SDKKey:                "",
			PlatformData: devcycle.PlatformData{
				SdkType:         "server",
				SdkVersion:      devcycle.VERSION,
				PlatformVersion: runtime.Version(),
				Platform:        "Go",
				Hostname:        hostname,
			},
			SDKConfig: SDKConfig{
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
