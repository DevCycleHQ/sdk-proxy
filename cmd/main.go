package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	sdkproxy "github.com/devcyclehq/sdk-proxy"
	"github.com/kelseyhightower/envconfig"
)

const (
	Version         = sdkproxy.Version
	EnvConfigFormat = `
This application can also be configured via the environment. The following environment
variables can be used:

{{printf "%-54s" "KEY"}}	{{ printf "%-11s" "TYPE" }}	DEFAULT	 REQUIRED	DESCRIPTION
{{range .}}{{usage_key . | printf "%-54s"}}	{{usage_type . | printf "%-11s"}}	{{usage_default .}}	{{usage_required . | printf "%5s" }}	{{usage_description .}}
{{end}}`
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "", "The path to a JSON config file.")

	flag.Usage = func() {
		log.Printf("DevCycle Local Bucketing Proxy Version %s\n", Version)

		log.Printf("Usage: %s [options]\n", os.Args[0])
		flag.PrintDefaults()
		_ = envconfig.Usagef(sdkproxy.EnvVarPrefix, &sdkproxy.FullEnvConfig{}, os.Stderr, EnvConfigFormat)
	}
	flag.Parse()

	config, err := sdkproxy.ParseConfig(configPath)
	if err != nil {
		log.Printf("Failed to parse config: %s", err)
		log.Fatal("Please either set the config path or set the environment variables")
	}

	if len(config.Instances) == 0 {
		log.Fatalf("No instances found in config. Use %s -config <path> to create a sample config file.", os.Args[0])
		return
	}
	// Create router for each instance
	for _, instance := range config.Instances {
		log.Printf("Creating bucketing proxy instance: %+v", instance)

		// Create client
		_, err = sdkproxy.NewBucketingProxyInstance(instance)
		if err != nil {
			log.Fatal(err)
		}
		defer func(path string) {
			err = os.Remove(path)
		}(instance.UnixSocketPath)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Use a buffered channel, so we don't miss any signals
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		// Block until a signal is received.
		s := <-c
		fmt.Printf("Received signal: %s, shutting down", s)

		for _, instance := range config.Instances {
			err := instance.Close()
			if err != nil {
				log.Printf("Failed to shut down instance: %s", err)
			}
		}

		cancel()
	}()

	<-ctx.Done()
}
