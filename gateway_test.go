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
		CacheSecond: 60,
		SuccessCode: 200,
	})
}

func TestGateway_Out(t *testing.T) {
	for i := 0; i < 300; i++ {
		ctx, err := gw.In(func() (accessId string, path string, cacheKey string) {
			return strconv.FormatInt(time.Now().UnixNano(), 10), "user/login", ""
		})

		if err != nil {
			t.Fatal(err)
		}

		time.Sleep(time.Millisecond)

		dur, qps, total, _ := gw.Out(ctx, func(code int, data []byte) (err error) {
			return nil
		}, 200, nil)

		t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)
	}

	t.Logf("%+v\n", gw.Info())
}

func TestGateway_In(t *testing.T) {
	ctx, err := gw.In(func() (accessId string, path string, cacheKey string) {
		return strconv.FormatInt(time.Now().UnixNano(), 10), "config/scrolls", "test_cache"
	})

	if err != nil {
		t.Fatal(err)
	}

	if ctx.Status() == StatusNo {
		t.Fatal("method is not avaliable")
	}

	dur, qps, total, err := gw.Out(ctx, func(code int, data []byte) (err error) {
		time.Sleep(time.Millisecond)
		return nil
	}, 200, []byte(ctx.AccessId()))

	if err != nil {
		t.Fatal(err)
	}

	t.Logf("dur:%v qps:%d total:%d\n", dur, qps, total)

	ctx, err = gw.In(func() (accessId string, path string, cacheKey string) {
		return strconv.FormatInt(time.Now().UnixNano(), 10), "config/scrolls", "test_cache"
	})
	if err != nil {
		t.Fatal(err)
	}

	if ctx.Status() == StatusCache {
		t.Logf("cache:%s\n", string(ctx.CacheData()))

		gw.Out(ctx, func(code int, data []byte) (err error) {
			time.Sleep(time.Millisecond)
			return nil
		}, 200, ctx.CacheData())
	}
}

func BenchmarkGateway_InWithCache(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.In(func() (accessId string, path string, cacheKey string) {
				return time.Now().String(), "config/scrolls", "scrolls"
			})

			if err != nil {
				b.Fatal(err)
			}

			var data []byte
			if ctx.Status() == StatusCache {
				data = ctx.CacheData()
			} else {
				time.Sleep(time.Microsecond)
				data = []byte(time.Now().String())
			}

			gw.Out(ctx, func(code int, data []byte) (err error) {
				time.Sleep(time.Nanosecond)
				return nil
			}, 200, data)
		}
	})
}

func BenchmarkGateway_InWithoutCache(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.In(func() (accessId string, path string, cacheKey string) {
				return time.Now().String(), "config/scrolls", ""
			})

			if err != nil {
				b.Fatal(err)
			}

			var data []byte
			if ctx.Status() == StatusCache {
				data = ctx.CacheData()
			} else {
				time.Sleep(time.Microsecond)
				data = []byte(time.Now().String())
			}

			gw.Out(ctx, func(code int, data []byte) (err error) {
				time.Sleep(time.Microsecond)
				return nil
			}, 200, data)
		}
	})
}

func BenchmarkGateway_InTimeoutWithCache(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.InTimeout(time.Millisecond, func() (accessId string, path string, cacheKey string) {
				return time.Now().String(), "config/scrolls", "scrolls"
			})

			if err == ErrTimeout {
				//b.Log(err)
				continue
			}

			var data []byte
			if ctx.Status() == StatusCache {
				data = ctx.CacheData()
			} else {
				time.Sleep(time.Microsecond)
				data = []byte(time.Now().String())
			}

			gw.Out(ctx, func(code int, data []byte) (err error) {
				time.Sleep(time.Nanosecond)
				return nil
			}, 200, data)
		}
	})
}

func BenchmarkGateway_InTimeoutWithoutCache(b *testing.B) {
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ctx, err := gw.InTimeout(time.Millisecond, func() (accessId string, path string, cacheKey string) {
				return time.Now().String(), "config/scrolls", ""
			})

			if err == ErrTimeout {
				//b.Log(err)
				continue
			}

			var data []byte
			if ctx.Status() == StatusCache {
				data = ctx.CacheData()
			} else {
				time.Sleep(time.Microsecond)
				data = []byte(time.Now().String())
			}

			gw.Out(ctx, func(code int, data []byte) (err error) {
				time.Sleep(time.Microsecond)
				return nil
			}, 200, data)
		}
	})
}
