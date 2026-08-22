package config

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/auth/s2s"
	"github.com/zeromicro/go-zero/zrpc"
)

type Config struct {
	zrpc.RpcServerConf
	Postgres struct {
		Datasource      string `json:",optional" secret:"true"`
		MaxOpenConns    int
		MaxIdleConns    int
		ConnMaxLifetime time.Duration
	}
	Kafka struct {
		Brokers          []string
		EventsTopic      string
		ReminderDueTopic string
		ConsumerGroup    string
		DLQTopic         string `json:",optional"`
	}
	// Expo push notifications configuration. See docs/push-notifications-design.md.
	Expo struct {
		// AccessToken is the optional Expo access token (raises rate limits).
		// Leave empty for unauthenticated sends (sufficient for low volume).
		AccessToken string `json:",optional" secret:"true"`
		// Enabled toggles push delivery. When false, the push sender is a no-op
		// so dev environments without push can run the service.
		Enabled bool `json:",optional,default=false"`
	}
	Email struct {
		Provider        string `json:",optional"`
		APIKey          string `json:",optional" secret:"true"`
		FromAddress     string `json:",optional"`
		FrontendBaseURL string `json:",optional"`
		Enabled         bool   `json:",optional,default=false"`
	}
	JWT         jwt.Config `json:",optional"`
	ServiceAuth s2s.Config `json:",optional"`
}
