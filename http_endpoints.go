package sdk_proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	devcycle_api "github.com/devcyclehq/go-server-sdk/v2/api"
	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) {
	c.Status(200)
}

func Variable() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := c.Value("devcycle").(*devcycle.Client)
		user := getUserFromBody(c)
		if user == nil {
			return
		}

		if c.Param("key") == "" {
			variables, err := client.AllVariables(*user)
			if err != nil {
				fmt.Println(err)
				c.JSON(http.StatusInternalServerError, gin.H{})
				return
			}
			c.JSON(http.StatusOK, variables)
			return
		}

		variable, err := client.Variable(*user, c.Param("key"), nil)
		if err != nil {
			fmt.Println(err)
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		if !variable.IsDefaulted {
			c.JSON(http.StatusOK, variable.BaseVariable)
		} else {
			c.JSON(http.StatusNotFound, gin.H{
				"message":    "Variable not found for key: " + c.Param("key"),
				"statusCode": http.StatusNotFound,
			})
		}
	}
}

func Feature() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := c.Value("devcycle").(*devcycle.Client)

		user := getUserFromBody(c)
		if user == nil {
			return
		}
		allFeatures, err := client.AllFeatures(*user)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		c.JSON(http.StatusOK, allFeatures)
	}
}

func Track() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := c.Value("devcycle").(*devcycle.Client)
		ofIdentifier := c.Request.Header.Get("X-DevCycle-OpenFeature-SDK")
		event := getEventFromBody(c)
		for _, e := range event.Events {
			if e.MetaData == nil {
				e.MetaData = make(map[string]interface{})
			}
			e.MetaData["sdkProxy"] = Version
			if ofIdentifier != "" {
				e.MetaData["sdkPlatform"] = ofIdentifier
			}
			_, err := client.Track(event.User.User, e)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{})
			}
		}
	}
}

func BatchEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		client := c.Value("devcycle").(*devcycle.Client)

		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading request body: " + err.Error()})
			return
		}
		defer c.Request.Body.Close()

		var batchEvents map[string]interface{}
		err = json.Unmarshal(body, &batchEvents)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error unmarshaling request body: " + err.Error()})
			return
		}

		batchArray, exists := batchEvents["batch"].([]interface{})
		if !exists {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Missing 'batch' key in request body"})
			return
		}

		for _, batchItem := range batchArray {
			batchMap, ok := batchItem.(map[string]interface{})
			if !ok {
				continue
			}

			events, ok := batchMap["events"].([]interface{})
			if !ok {
				continue
			}

			for i, eventInterface := range events {
				event, ok := eventInterface.(map[string]interface{})
				if !ok {
					continue
				}

				if _, exists := event["metaData"]; !exists {
					event["metaData"] = make(map[string]interface{})
				}

				metadata, ok := event["metaData"].(map[string]interface{})
				if !ok {
					event["metaData"] = make(map[string]interface{})
					metadata = event["metaData"].(map[string]interface{})
				}

				metadata["sdkProxy"] = Version
				events[i] = event
			}

			batchMap["events"] = events
		}

		modifiedBody, err := json.Marshal(batchEvents)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error marshaling modified request body: " + err.Error()})
			return
		}

		// Passthrough proxy to the configured events api endpoint.
		httpC := http.DefaultClient
		req, err := http.NewRequest("POST", client.DevCycleOptions.EventsAPIURI+"/v1/events/batch", bytes.NewBuffer(modifiedBody))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating request: " + err.Error()})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
		resp, err := httpC.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending request: " + err.Error()})
			return
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading response: " + err.Error()})
			return
		}
		defer resp.Body.Close()
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

func GetConfig(client *devcycle.Client, version ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		instance := c.Value("instance").(*ProxyInstance)

		if c.Param("sdkKey") == "" || !strings.HasSuffix(c.Param("sdkKey"), ".json") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		var ret, rawConfig []byte
		var etag, lm string
		var err error
		if client != nil {
			rawConfig, etag, lm, err = client.GetRawConfig()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{})
				return
			}
			if instance.SSEEnabled {
				secure := ""
				if instance.SSEHttps {
					secure = "s"
				}
				config := map[string]interface{}{}
				err = json.Unmarshal(rawConfig, &config)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{})
					return
				}
				hostname := ""
				if instance.SSEXForwardedOnly {
					xforwardedHost := c.Request.Header.Get("X-Forwarded-Host")
					xforwardedProto := c.Request.Header.Get("X-Forwarded-Proto")
					if xforwardedHost != "" {
						if xforwardedProto != "" {
							hostname = fmt.Sprintf("%s://%s", xforwardedProto, xforwardedHost)
						} else {
							if instance.SSEHttps {
								hostname = fmt.Sprintf("https://%s", xforwardedHost)
							}
						}
					} else {
						c.JSON(http.StatusForbidden, gin.H{})
						return
					}
				} else {

					hostname = fmt.Sprintf("http%s://%s:%d", secure, instance.SSEHostname, instance.HTTPPort)
					// This is the only indicator that a unix socket request was made
					if c.Request.RemoteAddr == "" {
						hostname = fmt.Sprintf("unix:%s", instance.UnixSocketPath)
					}
				}

				if val, ok := config["sse"]; ok {
					path := val.(map[string]interface{})["path"].(string)

					config["sse"] = devcycle_api.SSEHost{
						Hostname: hostname,
						Path:     path,
					}
				}

				ret, err = json.Marshal(config)
			} else {
				ret = rawConfig
			}
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{})
				return
			}
		} else if client == nil && len(version) > 0 {
			ret, etag, lm = instance.BypassSDKConfig(version[0])
		}
		c.Header("ETag", etag)
		c.Header("Last-Modified", lm)
		c.Data(http.StatusOK, "application/json", ret)
	}
}

func SSE() gin.HandlerFunc {
	return func(c *gin.Context) {
		instance := c.Value("instance").(*ProxyInstance)
		instance.sseServer.Handler(instance.SDKKey).ServeHTTP(c.Writer, c.Request)
	}
}

func getUserFromBody(c *gin.Context) *devcycle.User {
	if c.Param("key") != strings.ToLower(c.Param("key")) {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message":    "Variable Key must be lowercase",
			"statusCode": http.StatusBadRequest,
		})
		return nil
	}
	var user devcycle.User
	jsonBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message":    "Missing JSON body",
			"exception":  err.Error(),
			"statusCode": http.StatusBadRequest,
		})
		return nil
	}
	defer c.Request.Body.Close()
	err = json.Unmarshal(jsonBody, &user)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message":   "Invalid JSON body",
			"exception": err.Error(),

			"statusCode": http.StatusBadRequest,
		})
		return nil
	}
	return &user
}

func getEventFromBody(c *gin.Context) *devcycle.UserDataAndEventsBody {
	var event devcycle.UserDataAndEventsBody
	jsonBody, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message":    "Missing JSON body",
			"statusCode": http.StatusBadRequest,
		})
		return nil
	}
	defer c.Request.Body.Close()

	err = json.Unmarshal(jsonBody, &event)
	if err != nil {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"message":    "Invalid JSON body",
			"exception":  err.Error(),
			"statusCode": http.StatusBadRequest,
		})
		return nil
	}
	return &event
}
