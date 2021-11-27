package gateway

import (
	"math/rand"
	"testing"
	"time"
)

var (
	gw Gateway
)

func init() {
	gw = NewGateway(Option{
		Name:        "注册",
		Path:        "user/regis",
		SecondLimit: 1000,
	}, Option{
		Name:        "登录",
		Path:        "user/login",
		SecondLimit: 10000,
	})
}

func TestGateway_Info(t *testing.T) {
	for i := 0; i < 200; i++ {
		ctx, status := gw.Input(func() (accessId string, ip string, path string, param []byte) {
			return time.Now().String(), "", "user/regis", nil
		})
		t.Logf("status:%s\n", status)

		data, dur := gw.Output(ctx, func() []byte {
			return []byte(time.Now().String())
		})

		t.Logf("data:%s, dur:%v\n", string(data), dur)
	}

	t.Logf("%+v\n", gw.Info())
}

func BenchmarkGateway_Output(b *testing.B) {
	rand.Seed(time.Now().UnixNano())

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var ctx *Context
			if rand.Int()&1 == 0 {
				ctx, _ = gw.Input(func() (accessId string, ip string, path string, param []byte) {
					return time.Now().String(), "", "user/login", nil
				})
			} else {
				ctx, _ = gw.Input(func() (accessId string, ip string, path string, param []byte) {
					return time.Now().String(), "", "user/regis", nil
				})
			}

			gw.Output(ctx, func() []byte {
				return nil
			})
		}
	})
}
