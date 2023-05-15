package local_bucketing_proxy

import (
	"fmt"
	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
	"os"
	"strconv"
)

func NewBucketingProxyInstance(instance ProxyInstance) (err error) {
	options := instance.BuildDevCycleOptions()
	client, err := devcycle.NewClient(instance.SDKKey, options)
	if err != nil {
		return
	}

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	bucketingApiV1 := r.Group("/v1/")
	bucketingApiV1.Use(DevCycleAuthRequired())
	{
		bucketingApiV1.POST("/variables/:key", Variable(client))
		bucketingApiV1.POST("/variables", Variable(client))
		bucketingApiV1.POST("/features", Feature(client))
		bucketingApiV1.POST("/track", Track(client))
	}
	if instance.HTTPEnabled {
		if instance.HTTPPort == 0 {
			return fmt.Errorf("HTTP port must be set")
		}
		go r.Run(":" + strconv.Itoa(instance.HTTPPort))
		fmt.Println("HTTP server started on port " + strconv.Itoa(instance.HTTPPort))
	}
	if instance.UnixSocketEnabled {
		if _, err := os.Stat(instance.UnixSocketPath); err == nil {
			return fmt.Errorf("unix socket path %s already exists. Skipping instance creation", instance.UnixSocketPath)
		}
		go r.RunUnix(instance.UnixSocketPath)
		fmt.Println("Running on unix socket: " + instance.UnixSocketPath)
	}
	return nil
}
