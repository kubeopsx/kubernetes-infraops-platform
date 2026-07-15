package web

import (
	"github.com/gin-gonic/gin"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	ginprometheus "github.com/zsais/go-gin-prometheus"
	"net/http"
	"time"
)

func Run(httpAddress string, logger log.Logger) error {
	r := gin.New()

	m := make(map[string]interface{})
	m["logger"] = logger

	r.Use(middleware(m))

	p := ginprometheus.NewPrometheus("demo")
	p.Use(r)
	r.Use(gin.Logger())

	route(r)

	srv := &http.Server{
		Addr:           httpAddress,
		Handler:        r,
		ReadTimeout:    time.Second * 5,
		WriteTimeout:   time.Second * 5,
		MaxHeaderBytes: 1 << 20,
	}
	level.Info(logger).Log("msg", "web server available at", "httpAddress", httpAddress)
	err := srv.ListenAndServe()
	return err
}

func middleware(m map[string]interface{}) gin.HandlerFunc {
	return func(context *gin.Context) {
		for k, v := range m {
			context.Set(k, v)
			context.Next()
		}
	}
}
