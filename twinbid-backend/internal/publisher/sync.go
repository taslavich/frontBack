package publisher

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"log"
	"time"
	c "twinbid-backend/internal/pubcontract"
)

// A PostgreSQL advisory lock serializes sync workers, including rehydrate and resume.
func (s *Service) Sync(ctx context.Context) error {
	conn, e := s.DB.Conn(ctx)
	if e != nil {
		return e
	}
	defer conn.Close()
	var locked bool
	if e = conn.QueryRowContext(ctx, `SELECT pg_try_advisory_lock(871922133)`).Scan(&locked); e != nil || !locked {
		return e
	}
	defer func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if _, err := conn.ExecContext(unlockCtx, `SELECT pg_advisory_unlock(871922133)`); err != nil {
			log.Printf("publisher sync unlock: %v", err)
		}
	}()
	var epoch, state string
	if e = conn.QueryRowContext(ctx, `SELECT epoch,state FROM publisher_control WHERE id=1`).Scan(&epoch, &state); e != nil {
		return e
	}
	redisEpoch, e := s.Redis.Get(ctx, c.EpochKey).Result()
	if e != nil && e != redis.Nil {
		return e
	}
	policyEpoch, policyErr := s.Redis.HGet(ctx, c.Prefix+"{control}:blocks", "epoch").Result()
	if policyErr != nil && policyErr != redis.Nil {
		return policyErr
	}
	guardsLost := false
	if redisEpoch == epoch && policyEpoch == epoch && (s.lastGuardCheck.IsZero() || time.Since(s.lastGuardCheck) > 30*time.Second) {
		pipe := s.Redis.Pipeline()
		checks := make([]*redis.Cmd, 16384)
		for slot := range checks {
			checks[slot] = pipe.Eval(ctx, c.CheckGuardLua, []string{c.GuardKeyForSlot(slot)}, epoch)
		}
		_, checkErr := pipe.Exec(ctx)
		if checkErr != nil && checkErr != redis.Nil {
			return checkErr
		}
		for _, cmd := range checks {
			value, err := cmd.Result()
			if err != nil && err != redis.Nil {
				return err
			}
			if value != int64(1) {
				guardsLost = true
				break
			}
		}
		s.lastGuardCheck = time.Now()
	}
	if epoch == "" || redisEpoch != epoch || policyEpoch != epoch || guardsLost {
		// Losing an epoch means dynamic quotas may have been lost too. Rehydrate config,
		// but do NOT reopen delivery until an operator confirms restored dynamic state.
		epoch = uuid.NewString()
		state = "blocked"
		if _, e = conn.ExecContext(ctx, `UPDATE publisher_control SET epoch=$1,state='blocked' WHERE id=1`, epoch); e != nil {
			return e
		}
		if e = s.Redis.Set(ctx, c.EpochKey, epoch, 0).Err(); e != nil {
			return e
		}
		if e = s.Redis.Del(ctx, c.Prefix+"{control}:blocks").Err(); e != nil {
			return e
		}
		if e = s.Redis.HSet(ctx, c.Prefix+"{control}:blocks", "epoch", epoch).Err(); e != nil {
			return e
		}
		if e = s.Redis.Del(ctx, c.ReadyKey).Err(); e != nil {
			return e
		}
		pipe := s.Redis.Pipeline()
		for slot := 0; slot < 16384; slot++ {
			pipe.Eval(ctx, c.InitGuardLua, []string{c.GuardKeyForSlot(slot)}, epoch)
		}
		if _, e = pipe.Exec(ctx); e != nil {
			return e
		}
		s.lastGuardCheck = time.Now()
	}
	if e = s.Redis.Set(ctx, c.StateKey, state, 0).Err(); e != nil {
		return e
	}
	rows, e := conn.QueryContext(ctx, "SELECT "+codeColumns+" FROM ad_codes WHERE traffic_type IN ('adult','mainstream') AND (version>synced_version OR synced_epoch<>$1) ORDER BY updated_at LIMIT 500", epoch)
	if e != nil {
		return e
	}
	items := []Code{}
	for rows.Next() {
		v, err := scanCode(rows)
		if err != nil {
			rows.Close()
			return err
		}
		items = append(items, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range items {
		v.Epoch = epoch
		b, e := json.Marshal(v.Code)
		if e != nil {
			return e
		}
		if _, e = s.Redis.Eval(ctx, c.PutVersionLua, []string{c.CodeKey(v.ShortCode)}, v.Version, string(b), epoch).Result(); e != nil {
			return e
		}
		if _, e = conn.ExecContext(ctx, `UPDATE ad_codes SET synced_version=$2,synced_epoch=$3 WHERE ad_code_id=$1 AND version=$2`, v.ID, v.Version, epoch); e != nil {
			return e
		}
	}
	rows, e = conn.QueryContext(ctx, `SELECT hostname,blocked,reason,version FROM publisher_domain_blocks WHERE version>synced_version OR synced_epoch<>$1 ORDER BY updated_at LIMIT 500`, epoch)
	if e != nil {
		return e
	}
	type block struct {
		Hostname string `json:"hostname"`
		Blocked  bool   `json:"blocked"`
		Reason   string `json:"reason"`
		Version  int64  `json:"version"`
		Epoch    string `json:"epoch"`
	}
	blocks := []block{}
	for rows.Next() {
		v := block{Epoch: epoch}
		if e = rows.Scan(&v.Hostname, &v.Blocked, &v.Reason, &v.Version); e != nil {
			rows.Close()
			return e
		}
		blocks = append(blocks, v)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range blocks {
		b, _ := json.Marshal(v)
		if _, e = s.Redis.Eval(ctx, c.PutVersionLua, []string{c.BlockKey(v.Hostname)}, v.Version, string(b), epoch).Result(); e != nil {
			return e
		}
		blockValue := "0"
		if v.Blocked {
			blockValue = "1"
		}
		if e = s.Redis.HSet(ctx, c.Prefix+"{control}:blocks", v.Hostname, blockValue).Err(); e != nil {
			return e
		}
		if _, e = conn.ExecContext(ctx, `UPDATE publisher_domain_blocks SET synced_version=$2,synced_epoch=$3 WHERE hostname=$1 AND version=$2`, v.Hostname, v.Version, epoch); e != nil {
			return e
		}
	}
	var pending int
	if e = conn.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM ad_codes WHERE traffic_type IN ('adult','mainstream') AND (version>synced_version OR synced_epoch<>$1))+(SELECT count(*) FROM publisher_domain_blocks WHERE version>synced_version OR synced_epoch<>$1)`, epoch).Scan(&pending); e != nil {
		return e
	}
	if pending == 0 && state == "ready" {
		return s.Redis.Set(ctx, c.ReadyKey, epoch, 15*time.Second).Err()
	}
	return s.Redis.Del(ctx, c.ReadyKey).Err()
}
func (s *Service) Run(ctx context.Context) {
	t := time.NewTicker(s.Config.SyncInterval)
	defer t.Stop()
	maintenance := time.NewTicker(30 * time.Second)
	defer maintenance.Stop()
	for {
		cycle, cancel := context.WithTimeout(ctx, 10*time.Second)
		if e := s.Sync(cycle); e != nil {
			log.Printf("publisher sync: %v", e)
		}
		cancel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		case <-maintenance.C:
			cycle, cancel := context.WithTimeout(ctx, 20*time.Second)
			if e := s.Notify(cycle); e != nil {
				log.Printf("publisher notifications: %v", e)
			}
			cancel()
		}
	}
}
