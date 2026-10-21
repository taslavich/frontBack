package publisherconfig

import (
	"crypto/tls"
	"errors"
	"github.com/redis/go-redis/v9"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Config struct {
	RedisCluster        bool          `env:"PUBLISHER_REDIS_CLUSTER" env-default:"false"`
	RedisUsername       string        `env:"PUBLISHER_REDIS_USERNAME"`
	RedisAddrs          string        `env:"PUBLISHER_REDIS_ADDRS"`
	RedisDB             int           `env:"PUBLISHER_REDIS_DB" env-default:"-1"`
	RedisPassword       string        `env:"PUBLISHER_REDIS_PASSWORD"`
	RedisTLS            bool          `env:"PUBLISHER_REDIS_TLS" env-default:"false"`
	PublicURL           string        `env:"PUBLISHER_PUBLIC_URL"`
	PublicScriptURL     string        `env:"PUBLISHER_PUBLIC_SCRIPT_URL"`
	SyncInterval        time.Duration `env:"PUBLISHER_SYNC_INTERVAL" env-default:"2s"`
	RevenueSyncInterval time.Duration `env:"PUBLISHER_REVENUE_SYNC_INTERVAL" env-default:"30s"`
	RevenueSyncTimeout  time.Duration `env:"PUBLISHER_REVENUE_SYNC_TIMEOUT" env-default:"30s"`
}

func (c Config) Redis() (redis.UniversalClient, error) {
	if e := c.ValidateScriptURL(); e != nil {
		return nil, e
	}
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
	if strings.TrimSpace(c.RedisAddrs) == "" {
		return nil, errors.New("PUBLISHER_REDIS_ADDRS is required")
	}
	if c.RedisDB < 0 {
		return nil, errors.New("PUBLISHER_REDIS_DB must be explicitly set to a non-negative Redis database number")
	}
	if c.RedisDB == 5 {
		return nil, errors.New("PUBLISHER_REDIS_DB=5 is reserved for advertiser POP recovery")
	}
	if c.RedisCluster && c.RedisDB != 0 {
		return nil, errors.New("Redis Cluster only supports DB 0; use standalone Redis for Publisher database isolation")
	}
	a := strings.Split(c.RedisAddrs, ",")
	for i := range a {
		a[i] = strings.TrimSpace(a[i])
		if a[i] == "" {
			return nil, errors.New("PUBLISHER_REDIS_ADDRS contains empty address")
		}
	}
	o := &redis.UniversalOptions{Addrs: a, Password: c.RedisPassword, Username: c.RedisUsername, DB: c.RedisDB, ReadTimeout: time.Second, WriteTimeout: time.Second, PoolSize: 128}
	if c.RedisTLS {
		o.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	if c.RedisCluster {
		return redis.NewClusterClient(&redis.ClusterOptions{Addrs: a, Username: c.RedisUsername, Password: c.RedisPassword, TLSConfig: o.TLSConfig, ReadTimeout: o.ReadTimeout, WriteTimeout: o.WriteTimeout, PoolSize: o.PoolSize}), nil
	}
	return redis.NewUniversalClient(o), nil
}

// Optional stable SDK URL. An empty override keeps every existing embed unchanged.
func (c Config) ScriptURL() string {
	if c.PublicScriptURL != "" {
		return c.PublicScriptURL
	}
	return strings.TrimRight(c.PublicURL, "/") + "/static/adManager.js"
}
func (c Config) ValidateScriptURL() error {
	if c.PublicScriptURL == "" {
		return nil
	}
	u, e := url.Parse(c.PublicScriptURL)
	if e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.Path == "" || u.RawPath != "" || !regexp.MustCompile(`^/[A-Za-z0-9_/-]+\.js$`).MatchString(u.Path) || strings.Contains(u.Path, "//") || strings.Contains(u.Path, "..") || net.ParseIP(u.Hostname()) != nil || !strings.Contains(u.Hostname(), ".") || strings.HasSuffix(u.Hostname(), ".local") || (u.Port() != "" && u.Port() != "443") {
		return errors.New("PUBLISHER_PUBLIC_SCRIPT_URL must be an explicit HTTPS script URL without query/userinfo")
	}
	return nil
}
