package poprecovery

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"
)

const defaultMarkerPrefix = "pop-recovery:adjustment:"

// RedisApplier applies one immutable POP recovery adjustment to the existing
// ADV runtime spend keys in Redis DB 5. The marker has no TTL: it is the durable
// cross-system idempotency boundary for the crash window between Redis EXEC and
// the PostgreSQL status update.
type RedisApplier struct {
	client       *redis.Client
	markerPrefix string
	maxRetries   int
}

func NewRedisApplier(client *redis.Client, markerPrefix string, maxRetries int) *RedisApplier {
	markerPrefix = strings.TrimSpace(markerPrefix)
	if markerPrefix == "" {
		markerPrefix = defaultMarkerPrefix
	}
	if maxRetries <= 0 {
		maxRetries = 8
	}
	return &RedisApplier{client: client, markerPrefix: markerPrefix, maxRetries: maxRetries}
}

func (a *RedisApplier) Apply(ctx context.Context, adjustment Adjustment) (alreadyApplied bool, err error) {
	if a == nil || a.client == nil {
		return false, errors.New("POP recovery Redis client is nil")
	}
	if err := adjustment.Validate(); err != nil {
		return false, err
	}

	markerKey := a.markerPrefix + adjustment.ID
	userKey := "spent:user:" + adjustment.UserID
	campaignKey := "spent:campaign:" + adjustment.CampaignID

	for attempt := 0; attempt < a.maxRetries; attempt++ {
		alreadyApplied = false
		err = a.client.Watch(ctx, func(tx *redis.Tx) error {
			exists, err := tx.Exists(ctx, markerKey).Result()
			if err != nil {
				return fmt.Errorf("read POP recovery marker %s: %w", markerKey, err)
			}
			if exists > 0 {
				alreadyApplied = true
				return nil
			}

			// Validate both existing spend values before MULTI/EXEC. Only the marker
			// itself is WATCHed: ordinary realtime billing updates these hot spend
			// keys continuously, and watching them would create avoidable conflicts.
			// Redis EXEC continues after runtime command errors, so this preflight
			// prevents a pre-existing non-numeric spend value from allowing the other
			// increment and marker SET to commit partially.
			if err := validateRedisSpendValue(ctx, tx, userKey); err != nil {
				return err
			}
			if err := validateRedisSpendValue(ctx, tx, campaignKey); err != nil {
				return err
			}

			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.IncrByFloat(ctx, userKey, adjustment.DeltaFloat)
				pipe.IncrByFloat(ctx, campaignKey, adjustment.DeltaFloat)
				// No expiration by design. A pending PostgreSQL row may survive an
				// arbitrarily long outage after Redis EXEC and must never be replayed.
				pipe.Set(ctx, markerKey, adjustment.ID, 0)
				return nil
			})
			return err
		}, markerKey)
		if err == nil {
			return alreadyApplied, nil
		}
		if !errors.Is(err, redis.TxFailedErr) {
			return false, err
		}
	}
	return false, fmt.Errorf("POP recovery Redis transaction conflicted after %d retries", a.maxRetries)
}

func validateRedisSpendValue(ctx context.Context, tx *redis.Tx, key string) error {
	raw, err := tx.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read Redis spend %s: %w", key, err)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || value < 0 {
		return fmt.Errorf("invalid Redis spend value for %s: %q", key, raw)
	}
	return nil
}
