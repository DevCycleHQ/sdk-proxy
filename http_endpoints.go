package local_bucketing_proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
)

func Health(c *gin.Context) {
	c.Status(200)
}

func Variable(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
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

func Feature(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
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

func Track(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		event := getEventFromBody(c)
		for _, e := range event.Events {
			_, err := client.Track(event.User.User, e)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{})
			}
		}
	}
}

func BatchEvents(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Passthrough proxy to the configured events api endpoint.
		httpC := http.DefaultClient
		req, err := http.NewRequest("POST", client.DevCycleOptions.EventsAPIURI, c.Request.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error creating request"})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpC.Do(req)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error sending request"})
			return
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": "Error reading response"})
			return
		}
		defer resp.Body.Close()
		c.Data(resp.StatusCode, resp.Header.Get("Content-Type"), respBody)
	}
}

func GetConfig(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		rawConfig, etag, err := client.GetRawConfig()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{})
			return
		}
		c.Header("ETag", etag)
		c.Data(http.StatusOK, "application/json", rawConfig)
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
