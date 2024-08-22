package local_bucketing_proxy

import (
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

		event := getEventFromBody(c)
		for _, e := range event.Events {
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

		// Passthrough proxy to the configured events api endpoint.
		httpC := http.DefaultClient
		req, err := http.NewRequest("POST", client.DevCycleOptions.EventsAPIURI+"/v1/events/batch", c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating request, " + err.Error()})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", c.Request.Header.Get("Authorization"))
		resp, err := httpC.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending request, " + err.Error()})
			return
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading response, " + err.Error()})
			return
		}
		defer resp.Body.Close()
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

func GetConfig(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		instance := c.Value("instance").(*ProxyInstance)

		if c.Param("sdkKey") == "" || !strings.HasSuffix(c.Param("sdkKey"), ".json") {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		var ret []byte
		rawConfig, etag, err := client.GetRawConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		if instance.SSEEnabled {
			config := map[string]interface{}{}
			err = json.Unmarshal(rawConfig, &config)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{})
				return
			}
			hostname := fmt.Sprintf("http://%s:%d", instance.SSEHostname, instance.HTTPPort)
			// This is the only indicator that a unix socket request was made
			if c.Request.RemoteAddr == "" {
				hostname = fmt.Sprintf("unix:%s", instance.UnixSocketPath)
			}
			fmt.Println(c.Request)
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
		c.Header("ETag", etag)
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
