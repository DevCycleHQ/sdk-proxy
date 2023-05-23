package local_bucketing_proxy

import (
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"time"
)

type ProxyConfig struct {
	Instances []ProxyInstance `json:"instances"`
}

type SDKConfig struct {
	EventFlushIntervalMS         time.Duration `json:"eventFlushIntervalMS,omitempty" split_words:"true" desc:"The interval at which events are flushed to the events api in milliseconds."`
	ConfigPollingIntervalMS      time.Duration `json:"configPollingIntervalMS,omitempty" split_words:"true" desc:"The interval at which the SDK polls the config CDN for updates in milliseconds."`
	RequestTimeout               time.Duration `json:"requestTimeout,omitempty" split_words:"true" desc:"The timeout for requests to the config CDN and events API in milliseconds."`
	DisableAutomaticEventLogging bool          `json:"disableAutomaticEventLogging,omitempty" split_words:"true" default:"false" desc:"Whether to disable automatic event logging. Defaults to false."`
	DisableCustomEventLogging    bool          `json:"disableCustomEventLogging,omitempty" split_words:"true" default:"false" desc:"Whether to disable custom event logging. Defaults to false."`
	MaxEventQueueSize            int           `json:"maxEventsPerFlush,omitempty" split_words:"true" desc:"The maximum number of events to be in the queue before dropping events."`
	FlushEventQueueSize          int           `json:"minEventsPerFlush,omitempty" split_words:"true" desc:"The minimum number of events to be in the queue before flushing events."`
	ConfigCDNURI                 string        `json:"configCDNURI,omitempty" envconfig:"CONFIG_CDN_URI" desc:"The URI of the Config CDN - leave unspecified if not needing an outbound proxy."`
	EventsAPIURI                 string        `json:"eventsAPIURI,omitempty" envconvig:"EVENTS_API_URI" desc:"The URI of the Events API - leave unspecified if not needing an outbound proxy."`
}

type ProxyInstance struct {
	UnixSocketPath    string                `json:"unixSocketPath" envconfig:"UNIX_SOCKET_PATH" desc:"The path to the Unix socket."`
	HTTPPort          int                   `json:"httpPort" envconfig:"HTTP_PORT" default:"8080" desc:"The port to listen on for HTTP requests. Defaults to 8080."`
	UnixSocketEnabled bool                  `json:"unixSocketEnabled" envconfig:"UNIX_SOCKET_ENABLED" default:"false" desc:"Whether to enable the Unix socket. Defaults to false."`
	HTTPEnabled       bool                  `json:"httpEnabled" envconfig:"HTTP_ENABLED" default:"true" desc:"Whether to enable the HTTP server. Defaults to true."`
	SDKKey            string                `json:"sdkKey" required:"true" envconfig:"SDK_KEY" desc:"The Server SDK key to use for this instance."`
	PlatformData      devcycle.PlatformData `json:"platformData" required:"true"`
	SDKConfig         SDKConfig             `json:"sdkConfig" required:"true"`
	dvcClient         *devcycle.Client
}

func (i *ProxyInstance) Close() error {
	return i.dvcClient.Close()
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
