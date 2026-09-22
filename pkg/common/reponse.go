package common

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"net/http"
)

type RespJson struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func JsonResp(c *gin.Context, arg ...interface{}) (err error) {
	var (
		code    int
		message interface{}
		body    interface{}
	)
	if len(arg) == 1 {
		code = http.StatusOK
		message = arg[0]
	} else {
		code = arg[0].(int)
		message = arg[1]
	}
	if code == 200 {
		switch message.(type) {
		case string:
			body = RespJson{
				Message: message.(string),
			}
			c.JSON(http.StatusOK, body)
		default:
			c.JSON(http.StatusOK, message)
			body = message
		}
	} else {
		switch message.(type) {
		case string:
			body = RespJson{
				Error: message.(string),
			}
			c.JSON(code, body)
		case error:
			body = RespJson{
				Error: message.(error).Error(),
			}
			c.JSON(code, body)
		default:
			fmt.Println(message)
			body = RespJson{
				Error: "system type error plase ask admin for help",
			}
			c.JSON(code, body)
		}
	}
	return
}
