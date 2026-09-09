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
		// PublicBaseUrl is the browser-reachable base (e.g. https://api.example.com/files)
		// used when building public object URLs. Empty falls back to the Endpoint host.
		PublicBaseUrl string `json:",optional"`
	}
}
