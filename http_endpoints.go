package local_bucketing_proxy

import (
	"encoding/json"
	"fmt"
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"strings"
)

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
		return
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
