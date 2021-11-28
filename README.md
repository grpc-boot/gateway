# gateway
gateway

> 1.实例化

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

> 2.Option 解析

```go
type Option struct {
	Name        string `json:"name" yaml:"name"`                       //方法名称
	Path        string `json:"path" yaml:"path"`                       //方法路径
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`       //每秒限速，-1降级，0不限速，默认不限速
}
```