package sdk_proxy

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/devcyclehq/go-server-sdk/v2/api"
	"github.com/kr/pretty"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	defaultSDKConfig := SDKConfig{}
	defaultSDKConfig.Default()
	tests := []struct {
		name        string
		flag        string
		env         map[string]string
		expected    *ProxyConfig
		expectedErr string
	}{
		{
			name:        "no config",
			env:         map[string]string{},
			expected:    nil,
			expectedErr: "required key SDK_KEY missing value",
		},
		{
			name: "minimum env config",
			env: map[string]string{
				"SDK_KEY": "dvc-test-key",
			},
			expected: &ProxyConfig{
				Instances: []*ProxyInstance{
					{
						UnixSocketPath:        "",
						UnixSocketPermissions: "0755",
						HTTPPort:              8080,
						UnixSocketEnabled:     false,
						HTTPEnabled:           true,
						SSEEnabled:            true,
						SDKKey:                "dvc-test-key",
						LogFile:               "",
						PlatformData:          api.PlatformData{},
						SDKConfig:             SDKConfig{},
					},
				},
			},
		},
		{
			name: "all env config",
			env: map[string]string{
				"SDK_KEY":                                   "dvc-test-key",
				"DEBUG":                                     "True",
				"UNIX_SOCKET_PATH":                          "/tmp/dvc2.sock",
				"HTTP_PORT":                                 "1234",
				"UNIX_SOCKET_ENABLED":                       "True",
				"HTTP_ENABLED":                              "False",
				"PLATFORMDATA_SDKTYPE":                      "sdk type",
				"PLATFORMDATA_SDKVERSION":                   "v1.2.3",
				"PLATFORMDATA_PLATFORMVERSION":              "v2.3.4",
				"PLATFORMDATA_DEVICEMODEL":                  "device model",
				"PLATFORMDATA_PLATFORM":                     "platform",
				"PLATFORMDATA_HOSTNAME":                     "hostname",
				"SDKCONFIG_EVENT_FLUSH_INTERVAL_MS":         "60000",
				"SDKCONFIG_CONFIG_POLLING_INTERVAL_MS":      "120000",
				"SDKCONFIG_REQUEST_TIMEOUT":                 "3000",
				"SDKCONFIG_DISABLE_AUTOMATIC_EVENT_LOGGING": "True",
				"SDKCONFIG_DISABLE_CUSTOM_EVENT_LOGGING":    "True",
				"SDKCONFIG_MAX_EVENT_QUEUE_SIZE":            "123",
				"SDKCONFIG_FLUSH_EVENT_QUEUE_SIZE":          "456",
				"SDKCONFIG_CONFIG_CDN_URI":                  "https://example.com/config",
				"SDKCONFIG_EVENTS_API_URI":                  "https://example.com/events",
			},
			expected: &ProxyConfig{
				Instances: []*ProxyInstance{
					{
						UnixSocketPath:        "/tmp/dvc2.sock",
						HTTPPort:              1234,
						UnixSocketEnabled:     true,
						UnixSocketPermissions: "0755",
						HTTPEnabled:           false,
						SSEEnabled:            true,
						SDKKey:                "dvc-test-key",
						LogFile:               "",
						PlatformData: api.PlatformData{
							SdkType:         "sdk type",
							SdkVersion:      "v1.2.3",
							PlatformVersion: "v2.3.4",
							DeviceModel:     "device model",
							Platform:        "platform",
							Hostname:        "hostname",
						},
						SDKConfig: SDKConfig{
							EventFlushIntervalMS:         time.Minute.Milliseconds(),
							ConfigPollingIntervalMS:      (2 * time.Minute).Milliseconds(),
							RequestTimeout:               (3 * time.Second).Milliseconds(),
							DisableAutomaticEventLogging: true,
							DisableCustomEventLogging:    true,
							MaxEventQueueSize:            123,
							FlushEventQueueSize:          456,
							ConfigCDNURI:                 "https://example.com/config",
							EventsAPIURI:                 "https://example.com/events",
						},
					},
				},
			},
		},
		{
			name:        "bad JSON config from flag",
			flag:        "./testdata/invalid.config.json",
			env:         map[string]string{},
			expectedErr: "failed to parse config from JSON: invalid character ',' looking for beginning of object key string",
		},
		{
			name: "minimum config from flag",
			flag: "./testdata/minimum.config.json",
			env:  map[string]string{},
			expected: &ProxyConfig{
				Instances: []*ProxyInstance{
					{
						UnixSocketPath:    "",
						HTTPPort:          0,
						UnixSocketEnabled: false,
						HTTPEnabled:       false,
						SSEEnabled:        false,
						SDKKey:            "dvc-sample-key",
						LogFile:           "",
						PlatformData:      api.PlatformData{},
						SDKConfig:         defaultSDKConfig,
					},
				},
			},
		},
		{
			name: "minimum config from file env var",
			env: map[string]string{
				"CONFIG": "./testdata/minimum.config.json",
			},
			expected: &ProxyConfig{
				Instances: []*ProxyInstance{
					{
						UnixSocketPath:    "",
						HTTPPort:          0,
						UnixSocketEnabled: false,
						HTTPEnabled:       false,
						SDKKey:            "dvc-sample-key",
						LogFile:           "",
						PlatformData:      api.PlatformData{},
						SDKConfig:         defaultSDKConfig,
					},
				},
			},
		},
		{
			name: "all config from flag",
			flag: "./config.json.example",
			env:  map[string]string{},
			expected: &ProxyConfig{
				Instances: []*ProxyInstance{
					{
						UnixSocketPath:    "/tmp/devcycle.sock",
						HTTPPort:          8080,
						UnixSocketEnabled: false,
						HTTPEnabled:       true,
						SDKKey:            "dvc_YOUR_KEY_HERE",
						LogFile:           "",
						SSEEnabled:        false,
						PlatformData: api.PlatformData{
							SdkType:         "server",
							SdkVersion:      "2.10.2",
							PlatformVersion: "go1.20.3",
							DeviceModel:     "",
							Platform:        "Go",
							Hostname:        "localhost",
						},
						SDKConfig: SDKConfig{
							EventFlushIntervalMS:         (time.Second * 3).Milliseconds(),
							ConfigPollingIntervalMS:      (time.Second * 30).Milliseconds(),
							RequestTimeout:               (time.Second * 60).Milliseconds(),
							DisableAutomaticEventLogging: false,
							DisableCustomEventLogging:    false,
							MaxEventQueueSize:            10000,
							FlushEventQueueSize:          100,
							ConfigCDNURI:                 "https://config-cdn.devcycle.com",
							EventsAPIURI:                 "https://events.devcycle.com",
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for key, value := range test.env {
				_ = os.Setenv(EnvVarPrefix+"_"+key, value)
				defer func(key string) {
					_ = os.Unsetenv(EnvVarPrefix + "_" + key)
				}(key)
			}
			actual, err := ParseConfig(test.flag)
			if test.expectedErr != "" {
				require.EqualError(t, err, test.expectedErr)
			} else {
				require.NoError(t, err)
				if !assert.Equal(t, test.expected, actual) {
					pretty.Println(test.name, actual)
				}
			}
		})
	}
}

func TestProxyInstanceStringer(t *testing.T) {
	t.Run("SDK key redaction", func(t *testing.T) {
		// Create a test instance with a SDK key
		instance := ProxyInstance{
			SDKKey: "dvc_server_test_key_12345",
		}

		// Test the fmt.Stringer implementation
		// using fmt.Sprintf to invoke the String() method
		result := fmt.Sprintf("%+v", instance)

		// Verify that the SDK key is redacted
		if strings.Contains(result, "SDKKey:dvc_server_test_key_12345") {
			t.Error("String() method should redact the SDK key, but it was found in the output")
		}

		// Verify that the redacted text is present
		if !strings.Contains(result, "SDKKey:***REDACTED***") {
			t.Error("String() method should contain '***REDACTED***' for the SDK key")
		}

		// Verify that the original instance's SDK key is not modified
		if instance.SDKKey != "dvc_server_test_key_12345" {
			t.Error("Original instance's SDK key should not be modified by String() method")
		}
	})

	t.Run("empty SDK key redaction", func(t *testing.T) {
		instance := ProxyInstance{
			HTTPPort: 9090,
			SDKKey:   "",
		}
		result := fmt.Sprintf("%+v", instance)
		if !strings.Contains(result, "SDKKey:***REDACTED***") {
			t.Error("String() method should still show '***REDACTED***' even with empty SDK key")
		}
	})

	t.Run("value receiver doesn't modify original", func(t *testing.T) {
		original := ProxyInstance{
			SDKKey: "original_key_123",
		}

		// Call String method multiple times
		result1 := fmt.Sprintf("%+v", original)
		result2 := fmt.Sprintf("%+v", original)

		// Both should be identical
		if result1 != result2 {
			t.Error("Multiple calls to String() should return identical results")
		}

		// Original should be unchanged
		if original.SDKKey != "original_key_123" {
			t.Error("Original instance should not be modified by String() calls")
		}

		// Both results should contain redacted key
		if !strings.Contains(result1, "SDKKey:***REDACTED***") {
			t.Error("First String() call should contain redacted key")
		}
		if !strings.Contains(result2, "SDKKey:***REDACTED***") {
			t.Error("Second String() call should contain redacted key")
		}
	})

	t.Run("other fields remain visible", func(t *testing.T) {
		instance := ProxyInstance{
			UnixSocketPath:        "/tmp/test.sock",
			UnixSocketPermissions: "0755",
			HTTPPort:              8080,
			SDKKey:                "sensitive_key",
			LogFile:               "/tmp/test.log",
		}

		result := fmt.Sprintf("%+v", instance)

		// Verify that other fields are still present
		expectedFields := []string{
			"/tmp/test.sock",
			"0755",
			"8080",
			"/tmp/test.log",
		}

		for _, field := range expectedFields {
			if !strings.Contains(result, field) {
				t.Errorf("String() method should contain field value '%s', but it was not found", field)
			}
		}

		// SDK key should still be redacted
		if !strings.Contains(result, "SDKKey:***REDACTED***") {
			t.Error("SDK key should be redacted")
		}
		if strings.Contains(result, "sensitive_key") {
			t.Error("Sensitive key should not appear in string output")
		}
	})
}
