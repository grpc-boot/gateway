package main

import (
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grpc-boot/base"
	"github.com/grpc-boot/gateway"
)

var (
	conf   Conf
	gw     gateway.Gateway
	gwPath = `/gw`
)

const (
	HttpCode = `http:code`
)

var (
	serverIsBusy = []byte(`{"code": 408, "msg":"server is busy", data:{}}`)
	serverErr    = []byte(`{"code": 500, "msg":"internal server error", data:{}}`)
)

type Conf struct {
	GatewayOptions []gateway.Option `yaml:"gateway"`
}

func init() {
	err := base.YamlDecodeFile("app.yml", &conf)
	if err != nil {
		panic(err)
	}
	gw = gateway.NewGateway(conf.GatewayOptions...)
}

func withGateway() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path, accessTime := ctx.FullPath(), time.Now()
		if path == gwPath {
			return
		}

		status, err := gw.InTimeout(time.Second, path)

		switch status {
		case gateway.StatusNo:
			ctx.Abort()
			if err == nil { //降级
				ctx.Data(http.StatusRequestTimeout, "application/json", serverIsBusy)
				log.Println(gw.Out(accessTime, path, http.StatusRequestTimeout))
			} else { //异常
				ctx.Data(http.StatusInternalServerError, "application/json", serverErr)
				log.Println(gw.Out(accessTime, path, http.StatusInternalServerError))
			}
			return
		case gateway.StatusBusy: //超时
			ctx.Abort()
			ctx.Data(http.StatusRequestTimeout, "application/json", serverIsBusy)
			log.Println(gw.Out(accessTime, path, http.StatusRequestTimeout))
			return
		}

		ctx.Next()

		ctxStatus := ctx.GetInt(HttpCode)
		if ctxStatus < 1 {
			log.Println(gw.Out(accessTime, path, http.StatusOK))
			return
		}
		log.Println(gw.Out(accessTime, path, ctxStatus))
	}
}

func response(ctx *gin.Context, code int, result interface{}) {
	ctx.Set(HttpCode, code)
	ctx.JSON(code, result)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	router := gin.New()

	router.Use(withGateway())

	router.GET("/gw", func(ctx *gin.Context) {
		response(ctx, http.StatusOK, gw.Info())
	})

	router.GET("/user/regis", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))
		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, map[string]interface{}{
				"code": http.StatusOK,
				"msg":  "success",
				"data": make(map[string]string, 0),
			})
			return
		}

		response(ctx, http.StatusCreated, map[string]interface{}{
			"code": http.StatusCreated,
			"msg":  "success",
			"data": make(map[string]string, 0),
		})
	})

	router.GET("/user/login", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))
		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, map[string]interface{}{
				"code": http.StatusOK,
				"msg":  "success",
				"data": make(map[string]string, 0),
			})
			return
		}

		response(ctx, http.StatusForbidden, map[string]interface{}{
			"code": http.StatusForbidden,
			"msg":  "forbidden",
			"data": make(map[string]string, 0),
		})
	})

	router.GET("/user/info", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))
		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, map[string]interface{}{
				"code": http.StatusOK,
				"msg":  "success",
				"data": make(map[string]string, 0),
			})
			return
		}

		response(ctx, http.StatusNotFound, map[string]interface{}{
			"code": http.StatusNotFound,
			"msg":  "not exists",
			"data": make(map[string]string, 0),
		})
	})

	server := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	err := server.ListenAndServe()
	if err != nil {
		os.Exit(1)
	}
}
