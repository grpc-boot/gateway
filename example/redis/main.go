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
		`{"name": "获取用户信息","path": "/user/info","second_limit": 1000, "bucket_size": 10}`,
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
