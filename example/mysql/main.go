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
