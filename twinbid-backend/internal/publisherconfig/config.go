package publisherconfig

import (
	"crypto/tls"
	"errors"
	"github.com/redis/go-redis/v9"
	"net/url"
	"strings"
	"time"
)

type Config struct {
	RedisCluster        bool          `env:"PUBLISHER_REDIS_CLUSTER" env-default:"false"`
	RedisUsername       string        `env:"PUBLISHER_REDIS_USERNAME"`
	Enabled             bool          `env:"PUBLISHER_ENABLED" env-default:"false"`
	RedisAddrs          string        `env:"PUBLISHER_REDIS_ADDRS" env-default:"127.0.0.1:6379"`
	RedisPassword       string        `env:"PUBLISHER_REDIS_PASSWORD"`
	RedisTLS            bool          `env:"PUBLISHER_REDIS_TLS" env-default:"false"`
	PublicURL           string        `env:"PUBLISHER_PUBLIC_URL"`
	SyncInterval        time.Duration `env:"PUBLISHER_SYNC_INTERVAL" env-default:"2s"`
	RevenueSyncInterval time.Duration `env:"PUBLISHER_REVENUE_SYNC_INTERVAL" env-default:"30s"`
	RevenueSyncTimeout  time.Duration `env:"PUBLISHER_REVENUE_SYNC_TIMEOUT" env-default:"30s"`
}

func (c Config) Redis() (redis.UniversalClient, error) {
	u, e := url.Parse(c.PublicURL)
	if e != nil || u.Host == "" || u.User != nil || u.Scheme != "https" {
		return nil, errors.New("PUBLISHER_PUBLIC_URL must be a public HTTPS origin")
	}
	if c.SyncInterval <= 0 || c.SyncInterval > 30*time.Second {
		return nil, errors.New("PUBLISHER_SYNC_INTERVAL must be >0 and <=30s")
	}
	if c.RevenueSyncInterval <= 0 || c.RevenueSyncInterval > 5*time.Minute || c.RevenueSyncTimeout <= 0 || c.RevenueSyncTimeout > 5*time.Minute {
		return nil, errors.New("publisher revenue sync interval/timeout must be >0 and <=5m")
	}
	a := strings.Split(c.RedisAddrs, ",")
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
		if a[i] == "" {
			return nil, errors.New("PUBLISHER_REDIS_ADDRS contains empty address")
		}
	}
	o := &redis.UniversalOptions{Addrs: a, Password: c.RedisPassword, Username: c.RedisUsername, DB: 0, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 128}
	if c.RedisTLS {
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if c.RedisCluster {
		return redis.NewClusterClient(&redis.ClusterOptions{Addrs: a, Username: c.RedisUsername, Password: c.RedisPassword, TLSConfig: o.TLSConfig, ReadTimeout: o.ReadTimeout, WriteTimeout: o.WriteTimeout, PoolSize: o.PoolSize}), nil
	}
	return redis.NewUniversalClient(o), nil
}
