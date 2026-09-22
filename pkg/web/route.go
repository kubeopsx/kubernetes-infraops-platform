package web

import (
	"github.com/gin-gonic/gin"
	"github.com/kubeopsx/kubernetes-infraops-platform/pkg/web/action"
)

func route(route *gin.Engine) {
	// Keep /http-sd compatible with the original standalone http-sd service.
	route.GET("/http-sd", action.PrometheusHTTPServiceDiscovery)

	api := route.Group("/apis/v1")
	{
		api.GET("/ping", func(context *gin.Context) {
			context.String(200, "pong")
		})
		api.GET("/prometheus/http-sd", action.PrometheusHTTPServiceDiscovery)
		api.POST("/node-path", action.NodePathAdd)
		api.GET("/node-path", action.NodePathQuery)

		api.POST("/resource-mount", action.ResourceMount)
		api.DELETE("/resource-unmount", action.ResourceUnMount) // Ensure this matches the HTTP method and path

		api.POST("/resource-query", action.ResourceQuery)
		api.GET("/resource-group", action.ResourceGroup)
		api.POST("/resource-distribution", action.ResourceDistribution)

		api.POST("/logging", action.LogAdd)
		api.GET("/logging", action.LogGet)

		api.POST("/task", action.TaskAdd)
		api.GET("/task", action.TaskGet)
	}
}
