package app

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"twinbid-backend/internal/publisherconfig"
)

func publisherRedisTestConfig() publisherconfig.Config {
	return publisherconfig.Config{
		PublicURL:           "https://pub.example.com",
		SyncInterval:        2 * time.Second,
		RevenueSyncInterval: 30 * time.Second,
		RevenueSyncTimeout:  30 * time.Second,
	}
}

func TestPublisherRedisStartupRequiresExplicitAddress(t *testing.T) {
	cfg := publisherRedisTestConfig()
	client, err := requirePublisherRedis(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "PUBLISHER_REDIS_ADDRS is required") {
		if client != nil {
			_ = client.Close()
		}
		t.Fatalf("expected missing Publisher Redis address error, got %v", err)
	}
}

func TestPublisherRedisStartupRejectsUnavailableServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	cfg := publisherRedisTestConfig()
	cfg.RedisAddrs = addr
	client, err := requirePublisherRedis(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "publisher Redis unavailable at startup") {
		if client != nil {
			_ = client.Close()
		}
		t.Fatalf("expected a failed Publisher Redis startup probe, got %v", err)
	}
}
