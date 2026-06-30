package config

import "github.com/zeromicro/go-zero/zrpc"

type Config struct {
	zrpc.RpcServerConf
	MinIO struct {
		Endpoint      string
		AccessKey     string `json:",optional" secret:"true"`
		SecretKey     string `json:",optional" secret:"true"`
		UseSSL        bool
		DefaultBucket string
		Region        string `json:",optional"`
	}
}
