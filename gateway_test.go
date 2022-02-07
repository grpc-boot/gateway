package gateway

import (
	"testing"
	"time"
)

var (
	gw Gateway
)

func init() {
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
}

func TestGateway_In(t *testing.T) {
	accessTime := time.Now()
	status, exists := gw.In("config/scrolls")

	if !exists {
		t.Fatal("want true, got false")
	}

	if status == StatusNo {
		t.Fatal("method is not avaliable")
	}

	dur, qps, total, err := gw.Out(accessTime, "config/scrolls", 200)

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
}

func BenchmarkGateway_In(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			accessTime := time.Now()
			_, _ = gw.In("config/scrolls")

			gw.Out(accessTime, "config/scrolls", 200)
		}
	})
}
