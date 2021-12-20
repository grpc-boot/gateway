package gateway

import (
	"testing"
	"time"

	jsoniter "github.com/json-iterator/go"
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

func TestGateway_Out(t *testing.T) {
	for i := 0; i < 300; i++ {
		accessTime := time.Now()
		_, _, err := gw.InTimeout(time.Millisecond*100, "user/login")
		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Millisecond)

		dur, qps, total, _ := gw.Out(accessTime, "user/login", 200)

		t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
	}

	info, _ := jsoniter.Marshal(gw.Info())
	t.Logf("%s\n", string(info))
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

func BenchmarkGateway_InTimeout(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			accessTime := time.Now()
			_, _, err := gw.InTimeout(time.Millisecond*100, "user/login")

			if err != nil {
				b.Fatal(err)
			}

			gw.Out(accessTime, "user/login", 200)
		}
	})
}
