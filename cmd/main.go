package main

import (
	"encoding/json"
	"flag"
	"fmt"
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	lbproxy "github.com/devcyclehq/local-bucketing-proxy"
	"github.com/gin-gonic/gin"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

var (
	configJSONPath string
	config         lbproxy.ProxyConfig
)

func init() {
	if os.Getenv("DEVCYCLE_PROXY_CONFIG") != "" {
		configJSONPath = os.Getenv("DEVCYCLE_PROXY_CONFIG")
	} else {
		flag.StringVar(&configJSONPath, "config", "", "Path to config.json file")
	}

	if os.Getenv("DEVCYCLE_PROXY_DEBUG") == "" {
		gin.SetMode(gin.ReleaseMode)
	}
	flag.Parse()

	// Load config
	if configJSONPath == "" {
		instance, err := initializeProxyInstanceFromEnv()
		if err != nil {
			fmt.Println("Failed to initialize proxy instance from environment variables. Please either set the config path or set the environment variables.")
			return
		}
		defer func() {
			err = os.Remove(instance.UnixSocketPath)
		}()
		config = lbproxy.ProxyConfig{Instances: []lbproxy.ProxyInstance{*instance}}
	} else {
		configFile, err := os.ReadFile(configJSONPath)
		if err != nil {
			fmt.Println("Failed to read config file, writing an empty configuration file to the specified path.")
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
				fmt.Println("Failed to marshal config to JSON.")
				return
			}
			err = os.WriteFile(configJSONPath, bytes, 0644)
			if err != nil {
				fmt.Println("Failed to write config to file.")
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

	// Create router for each instance
	for _, instance := range config.Instances {
		// Create client
		err := lbproxy.NewBucketingProxyInstance(instance)
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
	prefix := "DEVCYCLE_PROXY_"
	instance := &lbproxy.ProxyInstance{}

	// Initialize UnixSocketPath from environment variable
	if unixSocketPath := os.Getenv(prefix + "UNIX_SOCKET_PATH"); unixSocketPath != "" {
		instance.UnixSocketPath = unixSocketPath
	} else {
		return nil, fmt.Errorf("environment variable %sUNIX_SOCKET_PATH is not set", prefix)
	}

	// Initialize HTTPPort from environment variable
	if httpPortStr := os.Getenv(prefix + "HTTP_PORT"); httpPortStr != "" {
		httpPort, err := strconv.Atoi(httpPortStr)
		if err != nil {
			return nil, fmt.Errorf("environment variable %sHTTP_PORT is not a valid integer: %s", prefix, err)
		}
		instance.HTTPPort = httpPort
	}

	// Initialize UnixSocketEnabled from environment variable
	if unixSocketEnabledStr := os.Getenv(prefix + "UNIX_SOCKET_ENABLED"); unixSocketEnabledStr != "" {
		unixSocketEnabled, err := strconv.ParseBool(unixSocketEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("environment variable %sUNIX_SOCKET_ENABLED is not a valid boolean: %s", prefix, err)
		}
		instance.UnixSocketEnabled = unixSocketEnabled
	}

	// Initialize HTTPEnabled from environment variable
	if httpEnabledStr := os.Getenv(prefix + "HTTP_ENABLED"); httpEnabledStr != "" {
		httpEnabled, err := strconv.ParseBool(httpEnabledStr)
		if err != nil {
			return nil, fmt.Errorf("environment variable %sHTTP_ENABLED is not a valid boolean: %s", prefix, err)
		}
		instance.HTTPEnabled = httpEnabled
	}

	// Initialize SDKKey from environment variable
	if sdkKey := os.Getenv(prefix + "SDK_KEY"); sdkKey != "" {
		instance.SDKKey = sdkKey
	} else {
		return nil, fmt.Errorf("environment variable %sSDK_KEY is not set", prefix)
	}

	// Initialize PlatformData from environment variables
	if platformData, err := initializePlatformDataFromEnv(prefix); err != nil {
		return nil, err
	} else {
		instance.PlatformData = platformData
	}

	// Initialize SDKConfig from environment variables
	if sdkConfig, err := initializeSDKConfigFromEnv(prefix); err != nil {
		return nil, err
	} else {
		instance.SDKConfig = sdkConfig
	}

	// Ensure that default values are set for any unset fields.
	instance.Default()

	return instance, nil
}

func initializePlatformDataFromEnv(prefix string) (devcycle.PlatformData, error) {
	platformData := devcycle.PlatformData{
		SdkType:         "",
		SdkVersion:      "",
		PlatformVersion: "",
		DeviceModel:     "",
		Platform:        "",
		Hostname:        "",
	}

	// Initialize Environment from environment variable
	if sdkType := os.Getenv(prefix + "PLATFORMDATA_SDK_TYPE"); sdkType != "" {
		platformData.SdkType = sdkType
	}
	if sdkVersion := os.Getenv(prefix + "PLATFORMDATA_SDK_VERSION"); sdkVersion != "" {
		platformData.SdkVersion = sdkVersion
	}
	if platformVersion := os.Getenv(prefix + "PLATFORMDATA_PLATFORM_VERSION"); platformVersion != "" {
		platformData.PlatformVersion = platformVersion
	}
	if deviceModel := os.Getenv(prefix + "PLATFORMDATA_DEVICE_MODEL"); deviceModel != "" {
		platformData.DeviceModel = deviceModel
	}
	if platform := os.Getenv(prefix + "PLATFORMDATA_PLATFORM"); platform != "" {
		platformData.Platform = platform
	}
	if hostname := os.Getenv(prefix + "PLATFORMDATA_HOSTNAME"); hostname != "" {
		platformData.Hostname = hostname
	}

	return platformData, nil
}

func initializeSDKConfigFromEnv(prefix string) (lbproxy.SDKConfig, error) {
	sdkConfig := lbproxy.SDKConfig{}

	// Initialize EventFlushIntervalMS from environment variable
	if eventFlushIntervalMSStr := os.Getenv(prefix + "EVENT_FLUSH_INTERVAL_MS"); eventFlushIntervalMSStr != "" {
		eventFlushIntervalMS, err := time.ParseDuration(eventFlushIntervalMSStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sEVENT_FLUSH_INTERVAL_MS is not a valid duration: %s", prefix, err)
		}
		sdkConfig.EventFlushIntervalMS = eventFlushIntervalMS
	} // Initialize ConfigPollingIntervalMS from environment variable
	if configPollingIntervalMSStr := os.Getenv(prefix + "CONFIG_POLLING_INTERVAL_MS"); configPollingIntervalMSStr != "" {
		configPollingIntervalMS, err := time.ParseDuration(configPollingIntervalMSStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sCONFIG_POLLING_INTERVAL_MS is not a valid duration: %s", prefix, err)
		}
		sdkConfig.ConfigPollingIntervalMS = configPollingIntervalMS
	}

	// Initialize RequestTimeout from environment variable
	if requestTimeoutStr := os.Getenv(prefix + "REQUEST_TIMEOUT"); requestTimeoutStr != "" {
		requestTimeout, err := time.ParseDuration(requestTimeoutStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sREQUEST_TIMEOUT is not a valid duration: %s", prefix, err)
		}
		sdkConfig.RequestTimeout = requestTimeout
	}

	// Initialize DisableAutomaticEventLogging from environment variable
	if disableAutomaticEventLoggingStr := os.Getenv(prefix + "DISABLE_AUTOMATIC_EVENT_LOGGING"); disableAutomaticEventLoggingStr != "" {
		disableAutomaticEventLogging, err := strconv.ParseBool(disableAutomaticEventLoggingStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sDISABLE_AUTOMATIC_EVENT_LOGGING is not a valid boolean: %s", prefix, err)
		}
		sdkConfig.DisableAutomaticEventLogging = disableAutomaticEventLogging
	}

	// Initialize DisableCustomEventLogging from environment variable
	if disableCustomEventLoggingStr := os.Getenv(prefix + "DISABLE_CUSTOM_EVENT_LOGGING"); disableCustomEventLoggingStr != "" {
		disableCustomEventLogging, err := strconv.ParseBool(disableCustomEventLoggingStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sDISABLE_CUSTOM_EVENT_LOGGING is not a valid boolean: %s", prefix, err)
		}
		sdkConfig.DisableCustomEventLogging = disableCustomEventLogging
	}

	// Initialize MaxEventQueueSize from environment variable
	if maxEventQueueSizeStr := os.Getenv(prefix + "MAX_EVENT_QUEUE_SIZE"); maxEventQueueSizeStr != "" {
		maxEventQueueSize, err := strconv.Atoi(maxEventQueueSizeStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sMAX_EVENT_QUEUE_SIZE is not a valid integer: %s", prefix, err)
		}
		sdkConfig.MaxEventQueueSize = maxEventQueueSize
	}

	// Initialize FlushEventQueueSize from environment variable
	if flushEventQueueSizeStr := os.Getenv(prefix + "FLUSH_EVENT_QUEUE_SIZE"); flushEventQueueSizeStr != "" {
		flushEventQueueSize, err := strconv.Atoi(flushEventQueueSizeStr)
		if err != nil {
			return sdkConfig, fmt.Errorf("environment variable %sFLUSH_EVENT_QUEUE_SIZE is not a valid integer: %s", prefix, err)
		}
		sdkConfig.FlushEventQueueSize = flushEventQueueSize
	}

	// Initialize ConfigCDNURI from environment variable
	if configCDNURI := os.Getenv(prefix + "CONFIG_CDN_URI"); configCDNURI != "" {
		sdkConfig.ConfigCDNURI = configCDNURI
	}

	// Initialize EventsAPIURI from environment variable
	if eventsAPIURI := os.Getenv(prefix + "EVENTS_API_URI"); eventsAPIURI != "" {
		sdkConfig.EventsAPIURI = eventsAPIURI
	}

	return sdkConfig, nil
}
