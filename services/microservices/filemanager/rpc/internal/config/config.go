package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/suleymanmyradov/growth-server/pkg/events/userdeletion"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	// ServiceAuth is the shared secret required on every incoming RPC call
	// (enforced by the s2s server interceptor).
	ServiceAuth s2s.Config `json:",optional"`
	MinIO       struct {
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
	// Postgres backs the file_objects registry (ownership + expiry). Optional:
	// when Datasource is empty, uploads/deletes still work against MinIO but
	// objects are untracked — no ownership checks and no retention sweeps.
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	// UserDeletion configures the user_deleted consumer that deletes every
	// object a user owns. Disabled when Topic is empty.
	UserDeletion userdeletion.Config `json:",optional"`
	// Cleanup controls the expired-object sweeper (deletes file_objects rows
	// past their expires_at, e.g. exports/ after 24h).
	Cleanup struct {
		// Interval between sweeps. Default 5m.
		Interval time.Duration `json:",optional"`
		// BatchSize bounds rows claimed per pass. Default 100.
		BatchSize int `json:",optional"`
	}
}
