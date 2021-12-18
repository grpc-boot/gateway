# gateway

### 1.实例化

```go
gw := NewGateway(Option{
		Name:        "登录",
		Path:        "user/login",
		SecondLimit: 100,
	}, Option{
		Name:        "获取轮播图",
		Path:        "config/scrolls",
		SecondLimit: 0,
	})
```

### 2.Option 解析

```go
type Option struct {
	Name        string `json:"name" yaml:"name"`                       //方法名称
	Path        string `json:"path" yaml:"path"`                       //方法路径
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`       //每秒限速，-1降级，0不限速，默认不限速
}
```

### 3.在`gin`中使用

> app.yml

```yaml
gateway:
  - name: '登录'
    path: '/user/login'
    second_limit: 0

  - name: '注册'
    path: '/user/regis'
    second_limit: -1

  - name: '获取用户信息'
    path: '/user/info'
    second_limit: 2000
```

> main.go

```go
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
	jsoniter "github.com/json-iterator/go"
)

var (
	conf Conf
	gw   gateway.Gateway
)

const (
	LogicCode = `logic:code`
)

type Response struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data"`
}

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
			if err == nil { //降级
				response(ctx, http.StatusRequestTimeout, "server is busy", nil)
				log.Println(gw.Out(accessTime, path, http.StatusRequestTimeout))
			} else { //异常
				response(ctx, http.StatusInternalServerError, "internal server error", nil)
			}

			ctx.Abort()
			return
		case gateway.StatusBusy: //超时
			response(ctx, http.StatusRequestTimeout, "server is busy", nil)
			log.Println(gw.Out(accessTime, path, http.StatusRequestTimeout))

			ctx.Abort()
			return
		}

		//默认设置为200
		ctx.Set(LogicCode, http.StatusOK)

		//handler
		ctx.Next()

		if exists {
			//网关出
			log.Println(gw.Out(accessTime, path, ctx.GetInt(LogicCode)))
		}
	}
}

func main() {
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
		time.Sleep(time.Millisecond * time.Duration(rand.Int63n(1000)))
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
```