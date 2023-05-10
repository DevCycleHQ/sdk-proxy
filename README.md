# DevCycle Local Bucketing Proxy

This is an implementation that uses our Go Server SDK to initialize and start multiple servers that emulate the response format of the 
bucketing-api server. This allows SDK's where implementing the WebAssembly bucketing library as a core isn't possible to 
benefit from the Local Bucketing benefits of the DevCycle platform.

## Usage

The application is delivered in multiple formats - a Docker image, a deb, and RPM package, and in a raw application format for local building and implementation.

The proxy handles two modes of operation - you can expose the HTTP server over a TCP port, or over Unix domain sockets.
The latter is recommended for servers that will deploy this in a fashion where the proxy is running on the same machine as the SDK, 
preventing the need for network calls.

The HTTP server mode is a 1:1 replacement for the bucketing-api - and can be used in place where there is no SDK in use as well.

### Docker
Please note that the docker container image in the repo is not the official one, and is only used for testing purposes.


## Options

