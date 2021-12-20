package main

import (
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grpc-boot/base"
	"github.com/grpc-boot/gateway"
	jsoniter "github.com/json-iterator/go"
)

var (
	gw gateway.Gateway
)

const (
	LogicCode = `logic:code`
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

func init() {
	optionFunc := gateway.OptionsWithJsonFile("app.json")
	//optionFunc := gateway.OptionsWithYamlFile("app.yml")
	gw = gateway.NewGateway(0, optionFunc)
}

func response(ctx *gin.Context, code int, msg string, data interface{}) {
	ctx.Set(LogicCode, code)

	result, _ := jsoniter.Marshal(Response{
		Code: code,
		Msg:  msg,
		Data: data,
	})

	ctx.Data(http.StatusOK, "application/json", result)
}

func withGateway() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path, accessTime := ctx.FullPath(), time.Now()
		status, exists, err := gw.InTimeout(time.Second, path)

		switch status {
		case gateway.StatusNo:
			var (
				code = http.StatusRequestTimeout
				msg  = "server is busy"
			)

			if err != nil { //异常
				code = http.StatusInternalServerError
				msg = "internal server error"
			}

			response(ctx, code, msg, nil)
			gw.Out(accessTime, path, code)
			ctx.Abort()
			return
		case gateway.StatusBusy: //超时
			response(ctx, http.StatusRequestTimeout, "server is busy", nil)
			gw.Out(accessTime, path, http.StatusRequestTimeout)
			ctx.Abort()
			return
		}

		//默认设置为200
		ctx.Set(LogicCode, http.StatusOK)

		//handler
		ctx.Next()

		if exists {
			//网关出
			duration, qps, total, er := gw.Out(accessTime, path, ctx.GetInt(LogicCode))
			base.Green("path:%s duration:%v qps:%d total:%d err:%v", path, duration, qps, total, er)
		}
	}
}

func main() {
	defer gw.Close()

	rand.Seed(time.Now().UnixNano())
	router := gin.New()

	router.Use(withGateway())

	router.GET("/gw", func(ctx *gin.Context) {
		response(ctx, http.StatusOK, "ok", gw.Info())
	})

	router.GET("/user/regis", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))
		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, "ok", nil)
			return
		}

		response(ctx, http.StatusCreated, "ok", nil)
	})

	router.GET("/user/login", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(10)))
		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, "ok", nil)
			return
		}

		response(ctx, http.StatusForbidden, "forbidden", nil)
	})

	router.GET("/user/info", func(ctx *gin.Context) {
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))

		if time.Now().Unix()%2 == 0 {
			response(ctx, http.StatusOK, "ok", nil)
			return
		}

		response(ctx, http.StatusNotFound, "not found", nil)
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
