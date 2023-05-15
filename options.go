package local_bucketing_proxy

import (
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"time"
)

type ProxyConfig struct {
	Instances []ProxyInstance `json:"instances"`
}

type SDKConfig struct {
	EventFlushIntervalMS         time.Duration `json:"eventFlushIntervalMS,omitempty"`
	ConfigPollingIntervalMS      time.Duration `json:"configPollingIntervalMS,omitempty"`
	RequestTimeout               time.Duration `json:"requestTimeout,omitempty"`
	DisableAutomaticEventLogging bool          `json:"disableAutomaticEventLogging,omitempty"`
	DisableCustomEventLogging    bool          `json:"disableCustomEventLogging,omitempty"`
	MaxEventQueueSize            int           `json:"maxEventsPerFlush,omitempty"`
	FlushEventQueueSize          int           `json:"minEventsPerFlush,omitempty"`
	ConfigCDNURI                 string
	EventsAPIURI                 string
}

type ProxyInstance struct {
	UnixSocketPath    string                `json:"unixSocketPath"`
	HTTPPort          int                   `json:"httpPort"`
	UnixSocketEnabled bool                  `json:"unixSocketEnabled"`
	HTTPEnabled       bool                  `json:"httpEnabled"`
	SDKKey            string                `json:"sdkKey"`
	PlatformData      devcycle.PlatformData `json:"platformData"`
	SDKConfig         SDKConfig             `json:"sdkConfig"`
}

func (i *ProxyInstance) BuildDevCycleOptions() *devcycle.Options {
	options := devcycle.Options{
		EnableEdgeDB:                 false,
		EnableCloudBucketing:         false,
		EventFlushIntervalMS:         i.SDKConfig.EventFlushIntervalMS,
		ConfigPollingIntervalMS:      i.SDKConfig.ConfigPollingIntervalMS,
		RequestTimeout:               i.SDKConfig.RequestTimeout,
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
	if i.UnixSocketEnabled && i.UnixSocketPath == "" {
		i.UnixSocketPath = "/tmp/devcycle.sock"
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
		c.RequestTimeout = 3000
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
