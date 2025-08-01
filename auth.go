package sdk_proxy

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func DevCycleAuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		sdkKey := ""
		if sdkKeyParam, hasSDKKeyParam := c.GetQuery("sdkKey"); hasSDKKeyParam {
			sdkKey = sdkKeyParam
		}
		if sdkKeyHeader := c.GetHeader("Authorization"); sdkKeyHeader != "" {
			sdkKey = sdkKeyHeader
		}
		if sdkKey == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message":    "Missing 'sdkKey' query parameter or 'Authorization' header",
				"statusCode": http.StatusUnauthorized,
			})
			c.Next()
			return
		}

		sdkKey = strings.ReplaceAll(sdkKey, "Bearer ", "")
		sdkKeyType := ""

		if strings.HasPrefix(sdkKey, "dvc") {
			sdkKeyType = strings.Split(sdkKey, "_")[1]
		} else {
			sdkKeyType = strings.Split(sdkKey, "-")[0]
		}

		switch sdkKeyType {
		case "server":
			break
		default:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message":    fmt.Sprintf("Only 'server', 'dvc_server' keys are supported by this API. Invalid key: %s", sdkKey),
				"statusCode": http.StatusUnauthorized,
			})
		}

		c.Set("dvc_sdk_key", sdkKey)
		c.Next()
	}
}
