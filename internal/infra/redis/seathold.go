package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
)

const (
	lockScript = `
local avail = redis.call('GET', KEYS[1])
if not avail then return -2 end
avail = tonumber(avail)
if avail < tonumber(ARGV[1]) then return -1 end
redis.call('DECRBY', KEYS[1], ARGV[1])
-- Hold count intentionally has no TTL: the zset score owns expiry so the
-- sweeper can always return the exact number of seats.
redis.call('SET', KEYS[2], ARGV[1])
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[3])
return avail - tonumber(ARGV[1])
`

	releaseScript = `
local n = redis.call('GET', KEYS[2])
if not n then return 0 end
redis.call('INCRBY', KEYS[1], n)
redis.call('DEL', KEYS[2])
redis.call('ZREM', KEYS[3], ARGV[1])
return 1
`

	paidScript = `
local n = redis.call('GET', KEYS[2])
if not n then return 0 end
redis.call('DEL', KEYS[2])
redis.call('ZREM', KEYS[3], ARGV[1])
return 1
`
)

type SeatHoldStore struct {
	client *goredis.Client
}

func NewSeatHoldStore(client *goredis.Client) *SeatHoldStore {
	return &SeatHoldStore{client: client}
}

func availKey(scheduleID uuid.UUID) string {
	return "seat:avail:" + scheduleID.String()
}

func holdKey(reservationID uuid.UUID) string {
	return "seat:hold:" + reservationID.String()
}

func holdsKey(scheduleID uuid.UUID) string {
	return "seat:holds:" + scheduleID.String()
}

// Lock atomically reserves n seats for a schedule.
// Returns remaining seats, -1 when unavailable, -2 when the counter is not initialized.
func (s *SeatHoldStore) Lock(ctx context.Context, scheduleID, reservationID uuid.UUID, n int, ttl time.Duration) (int64, error) {
	now := time.Now().UTC()
	result, err := s.client.Eval(ctx, lockScript,
		[]string{availKey(scheduleID), holdKey(reservationID), holdsKey(scheduleID)},
		n, now.Add(ttl).Unix(), reservationID.String(),
	).Int64()
	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *SeatHoldStore) Release(ctx context.Context, scheduleID, reservationID uuid.UUID) (bool, error) {
	result, err := s.client.Eval(ctx, releaseScript,
		[]string{availKey(scheduleID), holdKey(reservationID), holdsKey(scheduleID)},
		reservationID.String(),
	).Int64()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (s *SeatHoldStore) ConfirmPaid(ctx context.Context, scheduleID, reservationID uuid.UUID) error {
	_, err := s.client.Eval(ctx, paidScript,
		[]string{availKey(scheduleID), holdKey(reservationID), holdsKey(scheduleID)},
		reservationID.String(),
	).Result()
	return err
}

func (s *SeatHoldStore) InitIfMissing(ctx context.Context, scheduleID uuid.UUID, available int64) error {
	return s.client.SetNX(ctx, availKey(scheduleID), available, 0).Err()
}

func (s *SeatHoldStore) SetAvailable(ctx context.Context, scheduleID uuid.UUID, available int64) error {
	return s.client.Set(ctx, availKey(scheduleID), available, 0).Err()
}

func (s *SeatHoldStore) Available(ctx context.Context, scheduleID uuid.UUID) (int64, error) {
	value, err := s.client.Get(ctx, availKey(scheduleID)).Int64()
	if err == goredis.Nil {
		return 0, nil
	}
	return value, err
}

func (s *SeatHoldStore) ExpiredHolds(ctx context.Context, scheduleID uuid.UUID, limit int) ([]uuid.UUID, error) {
	members, err := s.client.ZRangeByScore(ctx, holdsKey(scheduleID), &goredis.ZRangeBy{
		Min:   "-inf",
		Max:   strconv.FormatInt(time.Now().UTC().Unix(), 10),
		Count: int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(members))
	for _, member := range members {
		id, err := uuid.Parse(member)
		if err != nil {
			_ = s.client.ZRem(ctx, holdsKey(scheduleID), member).Err()
			continue
		}
		out = append(out, id)
	}
	return out, nil
}

// Reconcile resets the availability counter from DB truth minus active holds.
func (s *SeatHoldStore) Reconcile(ctx context.Context, scheduleID uuid.UUID, quotaTotal, seatsSold int) error {
	members, err := s.client.ZRange(ctx, holdsKey(scheduleID), 0, -1).Result()
	if err != nil {
		return err
	}
	held := int64(0)
	for _, member := range members {
		id, err := uuid.Parse(member)
		if err != nil {
			continue
		}
		count, err := s.client.Get(ctx, holdKey(id)).Int64()
		if err == goredis.Nil {
			_ = s.client.ZRem(ctx, holdsKey(scheduleID), member).Err()
			continue
		}
		if err != nil {
			return fmt.Errorf("read hold %s: %w", id, err)
		}
		held += count
	}
	available := int64(quotaTotal-seatsSold) - held
	if available < 0 {
		available = 0
	}
	return s.SetAvailable(ctx, scheduleID, available)
}
