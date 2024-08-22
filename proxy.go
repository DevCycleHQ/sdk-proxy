package local_bucketing_proxy

import (
	"fmt"
	"github.com/devcyclehq/go-server-sdk/v2/api"
	"github.com/launchdarkly/eventsource"
	"io"
	"log"
	"os"
	"strconv"
	"time"

	devcycle "github.com/devcyclehq/go-server-sdk/v2"
	"github.com/gin-gonic/gin"
)

func NewBucketingProxyInstance(instance *ProxyInstance) (*ProxyInstance, error) {
	gin.DisableConsoleColor()
	logFile, err := os.OpenFile(instance.LogFile, os.O_CREATE|os.O_APPEND|os.O_RDWR, 0666)
	if err != nil {
		_ = fmt.Errorf("error opening log file: %s", err)
		return nil, err
	}
	mw := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(mw)
	gin.DefaultWriter = mw
	if instance.SSEEnabled {
		instance.sseEvents = make(chan api.ClientEvent, 100)
		instance.sseServer = eventsource.NewServer()
		instance.sseServer.ReplayAll = false
		eventsource.NewSliceRepository()
		go instance.EventRebroadcaster()
		log.Printf("Initialized SSE server at %s", instance.SSEHostname)
	}

	options := instance.BuildDevCycleOptions()
	client, err := devcycle.NewClient(instance.SDKKey, options)
	if err != nil {
		return nil, fmt.Errorf("error creating DevCycle client: %v", err)
	}
	instance.dvcClient = client

	r := newRouter(client, instance)

	if instance.HTTPEnabled {
		if instance.HTTPPort == 0 {
			return nil, fmt.Errorf("HTTP port must be set")
		}
		go func() {
			err = r.Run(":" + strconv.Itoa(instance.HTTPPort))
			if err != nil {
				log.Printf("Error running HTTP server: %s", err)
			}
		}()
		log.Printf("HTTP server started on port %d", instance.HTTPPort)
	}
	if instance.UnixSocketEnabled {
		if _, err = os.Stat(instance.UnixSocketPath); err == nil {
			return nil, fmt.Errorf("unix socket path %s already exists. Skipping instance creation", instance.UnixSocketPath)
		}
		err = nil
		go func() {
			err = r.RunUnix(instance.UnixSocketPath)
			if err != nil {
				log.Printf("Error running Unix socket server: %s", err)
			}
		}()
		_, err = os.Stat(instance.UnixSocketPath)
		for ; err != nil; _, err = os.Stat(instance.UnixSocketPath) {
			time.Sleep(1 * time.Second)
		}
		fileModeOctal, err := strconv.ParseUint(instance.UnixSocketPermissions, 8, 32)
		if err != nil {
			log.Printf("error parsing Unix socket permissions: %s", err)
			return nil, err
		}
		if err = os.Chmod(instance.UnixSocketPath, os.FileMode(fileModeOctal)); err != nil {
			log.Printf("Error setting Unix socket permissions: %s", err)
		}
		log.Printf("Running on unix socket: %s with file permissions %s", instance.UnixSocketPath, instance.UnixSocketPermissions)
	}
	return instance, err
}

// Add the DevCycle client to the request context
func devCycleMiddleware(client *devcycle.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("devcycle", client)
		c.Next()
	}
}

func sdkProxyMiddleware(instance *ProxyInstance) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("instance", instance)
		c.Next()
	}
}

func newRouter(client *devcycle.Client, instance *ProxyInstance) *gin.Engine {
	r := gin.New()

	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(devCycleMiddleware(client))
	r.Use(sdkProxyMiddleware(instance))
	r.GET("/healthz", Health)
	v1 := r.Group("/v1")
	v1.Use(DevCycleAuthRequired())
	{
		// Bucketing API
		v1.POST("/variables/:key", Variable())
		v1.POST("/variables", Variable())
		v1.POST("/features", Feature())
		v1.POST("/track", Track())
		// Events API
		v1.POST("/events", Track())
		v1.POST("/events/batch", BatchEvents())
	}
	configCDNv1 := r.Group("/config/v1")
	{
		configCDNv1.GET("/server/:sdkKey", GetConfig(nil, "v1"))
	}
	configCDNv2 := r.Group("/config/v2")
	{
		configCDNv2.GET("/server/:sdkKey", GetConfig(client))
	}
	r.GET("/event-stream", SSE())

	return r
}
