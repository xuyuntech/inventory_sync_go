package api

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type Api struct{}

func New() *Api {
	return &Api{}
}

func (a *Api) Run() error {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(cors.New(cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "HEAD"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "X-Requested-With", "X-Access-Token"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}))
	r.POST("/upload", a.Upload)
	return r.Run()
}

func RespErr(c *gin.Context, err error, msg ...string) {
	results := map[string]interface{}{
		"status": 1,
		"err":    err,
	}
	if len(msg) >= 1 {
		results["msg"] = msg[0]
	}
	c.JSON(200, results)
}

func Resp(c *gin.Context, results interface{}) {
	c.JSON(200, map[string]interface{}{
		"status": 0,
		"data":   results,
	})
}
