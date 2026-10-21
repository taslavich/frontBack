package config

import "testing"

func TestPublisherPasswordInheritsExistingRedisOnlyForSameHost(t *testing.T) {
	cfg := Config{}
	cfg.POPRecovery.RedisAddr = "buffer.twinbidexchange.com:7005"
	cfg.POPRecovery.RedisPassword = "existing-secret"
	cfg.Publisher.RedisAddrs = "buffer.twinbidexchange.com:7005"
	inheritPublisherRedisPassword(&cfg)
	if cfg.Publisher.RedisPassword != "existing-secret" {
		t.Fatalf("expected credentials to be reused for the same endpoint")
	}
}

func TestPublisherPasswordNeverOverridesExplicitCredential(t *testing.T) {
	cfg := Config{}
	cfg.POPRecovery.RedisAddr = "buffer.twinbidexchange.com:7005"
	cfg.POPRecovery.RedisPassword = "existing-secret"
	cfg.Publisher.RedisAddrs = "buffer.twinbidexchange.com:7005"
	cfg.Publisher.RedisPassword = "publisher-specific"
	inheritPublisherRedisPassword(&cfg)
	if cfg.Publisher.RedisPassword != "publisher-specific" {
		t.Fatalf("explicit publisher credential was overwritten")
	}
}

func TestPublisherPasswordNotInheritedForDifferentServer(t *testing.T) {
	cfg := Config{}
	cfg.POPRecovery.RedisAddr = "buffer.twinbidexchange.com:7005"
	cfg.POPRecovery.RedisPassword = "existing-secret"
	cfg.Publisher.RedisAddrs = "different.example.com:6379"
	inheritPublisherRedisPassword(&cfg)
	if cfg.Publisher.RedisPassword != "" {
		t.Fatalf("password was leaked to a different Redis endpoint")
	}
}

func TestPublisherPasswordNotInheritedWithoutPublisherAddr(t *testing.T) {
	cfg := Config{}
	cfg.POPRecovery.RedisAddr = "buffer.twinbidexchange.com:7005"
	cfg.POPRecovery.RedisPassword = "existing-secret"
	inheritPublisherRedisPassword(&cfg)
	if cfg.Publisher.RedisPassword != "" {
		t.Fatalf("password was inherited without a publisher address")
	}
}
