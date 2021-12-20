# gateway

<!-- TOC -->
- [gateway](#gateway)
    - [1.实例化](#1.实例化)
    - [2.Option解析](#2.Option解析)
    - [3.在gin中使用](#3.在gin中使用)
    - [4.用redis做options配置存储](#4.用redis做options配置存储)
    - [5.用mysql做options配置存储](#5.用mysql做options配置存储)
    
<!-- /TOC -->

### 1.实例化

```go
gw = NewGateway(0, func() (options []Option) {
    return []Option{
        Option{
            Name:        "登录",
            Path:        "user/login",
            SecondLimit: 100,
        },
        Option{
            Name:        "获取轮播图",
            Path:        "config/scrolls",
            SecondLimit: 0,
        },
    }
})
```

### 2.Option解析

```go
type Option struct {
	Name        string `json:"name" yaml:"name"`                       //方法名称
	Path        string `json:"path" yaml:"path"`                       //方法路径
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`       //每秒限速，-1降级，0不限速，默认不限速
}
```

### 3.在gin中使用

> app.yml

```yaml
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
```

### 4.用redis做options配置存储

```go
package main

import (
	"time"

	"github.com/grpc-boot/base"
	"github.com/grpc-boot/gateway"

	redigo "github.com/garyburd/redigo/redis"
	jsoniter "github.com/json-iterator/go"
)

const (
	hashKey   = "gateway:options"
	redisAddr = `127.0.0.1:6379`
)

var (
	redisPool *redigo.Pool
	gw        gateway.Gateway
)

func init() {
	dialOptions := []redigo.DialOption{
		redigo.DialReadTimeout(time.Millisecond * 500),
	}

	redisPool = &redigo.Pool{
		MaxIdle:   1,
		MaxActive: 1,
		Dial: func() (redigo.Conn, error) {
			return redigo.Dial("tcp", redisAddr, dialOptions...)
		},
		TestOnBorrow: func(c redigo.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
	}

	conn := redisPool.Get()
	defer conn.Close()

	args := []interface{}{
		hashKey,
		1,
		`{"name": "登录","path": "/user/login","second_limit": 0}`,
		2,
		`{"name": "注册","path": "/user/regis","second_limit": -1}`,
		3,
		`{"name": "获取用户信息","path": "/user/info","second_limit": 1000}`,
	}

	_, err := conn.Do("HMSET", args...)
	if err != nil {
		base.RedFatal("set redis config err:%s", err.Error())
	}
}

func main() {
	gw = gateway.NewGateway(time.Second, gateway.OptionsWithRedis(redisPool, hashKey))

	go func() {
		for {
			info, _ := jsoniter.Marshal(gw.Info())
			base.Green("%s", string(info))
			time.Sleep(time.Second)
		}
	}()

	var wa chan struct{}
	<-wa

	gw.Close()
}

```

### 5.用mysql做options配置存储

```go
package main

import (
	"database/sql"
	"time"

	"github.com/grpc-boot/base"
	"github.com/grpc-boot/gateway"

	_ "github.com/go-sql-driver/mysql"
	jsoniter "github.com/json-iterator/go"
)

/**
CREATE TABLE `gateway` (
  `id` int unsigned NOT NULL AUTO_INCREMENT COMMENT '主键',
  `name` varchar(32) CHARACTER SET utf8 COLLATE utf8_general_ci DEFAULT '' COMMENT '名称',
  `path` varchar(255) CHARACTER SET utf8 COLLATE utf8_general_ci DEFAULT '' COMMENT '路径',
  `second_limit` int unsigned DEFAULT '5000' COMMENT '每秒请求数',
  PRIMARY KEY (`id`),
  UNIQUE KEY `path` (`path`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
*/

const (
	tableName = "gateway"
	dsn       = `root:123456@tcp(127.0.0.1:3306)/dd?timeout=5s&readTimeout=6s`
)

var (
	db *sql.DB
	gw gateway.Gateway
)

func init() {
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		base.RedFatal("open mysql err:%s", err.Error())
	}

	db.SetConnMaxLifetime(time.Second * 100)
	db.SetConnMaxIdleTime(time.Second * 100)
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(5)

	args := []interface{}{
		"登录", "/user/login", "0",
		"注册", "/user/regis", "-1",
		"获取用户信息", "/user/info", "1000",
	}

	_, err = db.Exec("INSERT IGNORE INTO `gateway`(`name`,`path`,`second_limit`)VALUES(?,?,?),(?,?,?),(?,?,?)", args...)
	if err != nil {
		base.RedFatal("insert config err:%s", err.Error())
	}
}

func main() {
	gw = gateway.NewGateway(time.Second, gateway.OptionsWithDb(db, tableName))

	go func() {
		for {
			info, _ := jsoniter.Marshal(gw.Info())
			base.Green("%s", string(info))
			time.Sleep(time.Second)
		}
	}()

	var wa chan struct{}
	<-wa

	gw.Close()
}
```