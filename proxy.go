package local_bucketing_proxy

import (
	"fmt"
	"log"
	"os"
	"strconv"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
)

func NewBucketingProxyInstance(instance *ProxyInstance) (*ProxyInstance, error) {
	options := instance.BuildDevCycleOptions()
	client, err := devcycle.NewClient(instance.SDKKey, options)
	instance.dvcClient = client
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
			return nil, fmt.Errorf("HTTP port must be set")
		}
		go func() {
			err := r.Run(":" + strconv.Itoa(instance.HTTPPort))
			if err != nil {
				log.Printf("Error running HTTP server: %s", err)
			}
		}()
		log.Printf("HTTP server started on port %d", instance.HTTPPort)
	}
	if instance.UnixSocketEnabled {
		if _, err := os.Stat(instance.UnixSocketPath); err == nil {
			return nil, fmt.Errorf("unix socket path %s already exists. Skipping instance creation", instance.UnixSocketPath)
		}
		go func() {
			err := r.RunUnix(instance.UnixSocketPath)
			if err != nil {
				log.Printf("Error running Unix socket server: %s", err)
			}
		}()
		log.Printf("Running on unix socket: %s", instance.UnixSocketPath)
	}
	return instance, err
}
