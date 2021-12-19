package main

import (
	"io/ioutil"
	"net/http"

	"github.com/grpc-boot/base"
)

func main() {
	request(20, "http://127.0.0.1:8080/user/login")
	var wa chan struct{}
	<-wa
}

func request(workerNum int, url string) {
	for i := 0; i < workerNum; i++ {
		go func() {
			for {
				response, err := http.Get(url)
				if err != nil {
					base.Red("request %s err:%s", url, err.Error())
					continue
				}

				defer response.Body.Close()
				body, _ := ioutil.ReadAll(response.Body)

				base.Green(string(body))
			}
		}()
	}
}
