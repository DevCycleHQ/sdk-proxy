package local_bucketing_proxy

import (
	"fmt"
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

	options := instance.BuildDevCycleOptions()
	client, err := devcycle.NewClient(instance.SDKKey, options)
	instance.dvcClient = client
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.GET("/healthz", Health)
	v1 := r.Group("/v1")
	v1.Use(DevCycleAuthRequired())
	{
		// Bucketing API
		v1.POST("/variables/:key", Variable(client))
		v1.POST("/variables", Variable(client))
		v1.POST("/features", Feature(client))
		v1.POST("/track", Track(client))
		// Events API
		v1.POST("/events", Track(client))
		v1.POST("/events/batch", BatchEvents(client))
	}
	configCDNv1 := r.Group("/config/v1")
	{
		configCDNv1.GET("/server/:sdkKey", GetConfig(client))
	}

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
