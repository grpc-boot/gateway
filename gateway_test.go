package gateway

import (
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
		accessTime := time.Now()
		_, err := gw.In("user/login")

		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Millisecond)

		dur, qps, total, _ := gw.Out(accessTime, "user/login", 200)

		t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
	}

	t.Logf("%+v\n", gw.Info())
}

func TestGateway_In(t *testing.T) {
	accessTime := time.Now()
	status, err := gw.In("config/scrolls")

	if err != nil {
		t.Fatal(err)
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
			_, err := gw.In("config/scrolls")

			if err != nil {
				b.Fatal(err)
			}

			gw.Out(accessTime, "config/scrolls", 200)
		}
	})
}

func BenchmarkGateway_InTimeout(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			accessTime := time.Now()
			_, err := gw.InTimeout(time.Millisecond*100, "config/scrolls")

			if err != nil {
				b.Fatal(err)
			}

			gw.Out(accessTime, "config/scrolls", 200)
		}
	})
}
