package gateway

import (
	"strconv"
	"testing"
	"time"
)

var (
	gw Gateway
)

func init() {
	gw = NewGateway(Option{
		Name:        "登录",
		Path:        "user/login",
		SecondLimit: 100,
	}, Option{
		Name:        "获取轮播图",
		Path:        "config/scrolls",
		SecondLimit: 0,
	})
}

func TestGateway_Out(t *testing.T) {
	for i := 0; i < 300; i++ {
		ctx, err := gw.In(strconv.FormatInt(time.Now().UnixNano(), 10), "user/login")

		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Millisecond)

		dur, qps, total, _ := gw.Out(ctx, 200, nil)

		t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
	}

	t.Logf("%+v\n", gw.Info())
}

func TestGateway_In(t *testing.T) {
	ctx, err := gw.In(strconv.FormatInt(time.Now().UnixNano(), 10), "config/scrolls")

	if err != nil {
		t.Fatal(err)
	}

	if ctx.Status() == StatusNo {
		t.Fatal("method is not avaliable")
	}

	dur, qps, total, err := gw.Out(ctx, 200, []byte(ctx.AccessId()))

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
}

func BenchmarkGateway_In(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.In(time.Now().String(), "config/scrolls")

			if err != nil {
				b.Fatal(err)
			}

			gw.Out(ctx, 200, []byte(time.Now().String()))
		}
	})
}

func BenchmarkGateway_InTimeout(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.InTimeout(time.Millisecond, time.Now().String(), "config/scrolls")

			if err == ErrTimeout {
				continue
			}

			gw.Out(ctx, 200, []byte(time.Now().String()))
		}
	})
}
