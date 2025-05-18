package redislock

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	defaultValuer = func() string {
		return uuid.New().String()
	}

	//go:embed script/unlock.lua
	unlockLua string
	//go:embed script/refresh.lua
	refreshLua string

	ErrFailedToPreemptLock = errors.New("lock: Failed Lock Grab")
)

type Client struct {
	client redis.Cmdable
	valuer func() string
}

func NewClient(client redis.Cmdable) *Client {
	return &Client{
		client: client,
		valuer: defaultValuer,
	}
}

func (c *Client) SetValuer(valuer func() string) {
	c.valuer = valuer
}

func (c *Client) Lock(ctx context.Context, key string, expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	val := c.valuer()

	var timer *time.Timer
	defer func() {
		if timer != nil {
			timer.Stop()
		}
	}()

	for {
		lctx, cancel := context.WithTimeout(ctx, timeout)
		res, err := c.client.SetNX(lctx, key, val, expiration).Result()
		cancel()
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		if res {
			return newLock(c.client, key, val, expiration), nil
		}
		interval, ok := retry.Next()
		if !ok {
			if err != nil {
				err = fmt.Errorf("last retry failed: %s", err)
			} else {
				err = fmt.Errorf("the lock is being held: %s", ErrFailedToPreemptLock)
			}
			return nil, fmt.Errorf("retry lock: retries are exhausted, %w", err)
		}
		if timer == nil {
			timer = time.NewTimer(interval)
		} else {
			timer.Reset(interval)
		}
		select {
		case <-timer.C:
			// retry
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

func (c *Client) TryLock(ctx context.Context, key string, expiration time.Duration, timeout time.Duration) (*Lock, error) {
	val := c.valuer()

	lctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	res, err := c.client.SetNX(lctx, key, val, expiration).Result()
	if err != nil {
		return nil, err
	}
	if !res {
		// Someone has already added a lock or someone just happened to add a lock together,
		// but the competition failed.
		return nil, ErrFailedToPreemptLock
	}

	return newLock(c.client, key, val, expiration), nil
}

type Lock struct {
	client     redis.Cmdable
	key        string
	value      string
	expiration time.Duration
	unLockOnce sync.Once
}

func newLock(client redis.Cmdable, key string, value string, expiration time.Duration) *Lock {
	return &Lock{
		client:     client,
		key:        key,
		value:      value,
		expiration: expiration,
	}
}

func (l *Lock) Refresh(ctx context.Context) error {
	// TODO
	panic("")
}

func (l *Lock) AutoRefresh(ctx context.Context) error {
	// TODO
	panic("")
}

func (l *Lock) Unlock(ctx context.Context) error {
	// TODO
	panic("")
}
