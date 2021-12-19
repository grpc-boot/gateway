package gateway

import (
	"database/sql"

	"github.com/grpc-boot/base"

	redigo "github.com/garyburd/redigo/redis"
	jsoniter "github.com/json-iterator/go"
)

// OptionsFunc 加载配置函数
type OptionsFunc func() (options []Option)

// Option 方法配置
type Option struct {
	Name        string `json:"name" yaml:"name"`
	Path        string `json:"path" yaml:"path"`
	SecondLimit int    `json:"second_limit" yaml:"second_limit"`
}

// OptionsWithDb 从数据库表加载配置
func OptionsWithDb(db *sql.DB, tableName string) (optionsFunc OptionsFunc) {
	return func() (options []Option) {
		rows, err := db.Query("SELECT * FROM " + tableName)
		if err != nil {
			base.Red("OptionsWithMysql: get options form mysql table [%s] err:%s", tableName, err.Error())
			return
		}

		defer rows.Close()

		var id uint32

		options = make([]Option, 0, 32)
		for rows.Next() {
			var option Option
			if err = rows.Scan(&id, &option.Name, &option.Path, &option.SecondLimit); err != nil {
				continue
			}
			options = append(options, option)
		}
		return options
	}
}

// OptionsWithRedis 从redis哈希Key加载配置
func OptionsWithRedis(red *redigo.Pool, hashKey string) (optionsFunc OptionsFunc) {
	return func() (options []Option) {
		conn := red.Get()
		defer conn.Close()

		confList, err := redigo.StringMap(conn.Do("HGETALL", hashKey))
		if err != nil {
			base.Red("OptionsWithRedis: get options form redis key [%s] err:%s", hashKey, err.Error())
			return
		}

		if len(confList) < 1 {
			return
		}

		options = make([]Option, 0, len(confList))
		for _, conf := range confList {
			var option Option
			if err = jsoniter.UnmarshalFromString(conf, &option); err != nil {
				continue
			}
			options = append(options, option)
		}

		return options
	}
}

// OptionsWithJsonFile 从Json文件加载配置
func OptionsWithJsonFile(filepath string) (optionsFunc OptionsFunc) {
	return func() (options []Option) {
		err := base.YamlDecodeFile(filepath, &options)
		if err != nil {
			base.Red("OptionsWithJsonFile: load json file [%s] err:%s", filepath, err.Error())
		}
		return
	}
}

// OptionsWithYamlFile 从Yaml文件加载配置
func OptionsWithYamlFile(filepath string) (optionsFunc OptionsFunc) {
	return func() (options []Option) {
		err := base.YamlDecodeFile(filepath, &options)
		if err != nil {
			base.Red("OptionsWithYamlFile: load yaml file [%s] err:%s", filepath, err.Error())
		}
		return
	}
}
