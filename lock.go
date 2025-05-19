package redislock

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"golang.org/x/sync/singleflight"
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
	// ErrLockNotHold typically occurs when you expect that you are supposed to hold the lock, but you don't
	// For example, you might get this error when you try to release a lock.
	// This generally means that someone has bypassed rlock's controls and manipulated Redis directly.
	ErrLockNotHold = errors.New("rlock: did not hold lock")
)

type Client struct {
	client redis.Cmdable
	g      singleflight.Group
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

func (c *Client) SingleflightLock(ctx context.Context, key string, expiration time.Duration, timeout time.Duration, retry RetryStrategy) (*Lock, error) {
	for {
		flag := false
		res := c.g.DoChan(key, func() (interface{}, error) {
			flag = true
			return c.Lock(ctx, key, expiration, timeout, retry)
		})
		select {
		case result := <-res:
			if flag {
				c.g.Forget(key)
				if result.Err != nil {
					return nil, result.Err
				}
				return result.Val.(*Lock), nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
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
	unlock     chan struct{}
	unLockOnce sync.Once
}

func newLock(client redis.Cmdable, key string, value string, expiration time.Duration) *Lock {
	return &Lock{
		client:     client,
		key:        key,
		value:      value,
		expiration: expiration,
		unlock:     make(chan struct{}, 1),
	}
}

func (l *Lock) Refresh(ctx context.Context) error {
	res, err := l.client.Eval(ctx, refreshLua, []string{l.key}, l.value, l.expiration.Seconds()).Int64()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}

func (l *Lock) AutoRefresh(interval time.Duration, timeout time.Duration) error {
	ticker := time.NewTicker(interval)
	ch := make(chan struct{}, 1)
	defer func() {
		ticker.Stop()
		close(ch)
	}()
	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			// timeout here to keep trying
			if errors.Is(err, context.DeadlineExceeded) {
				// Because there are two possible places to write data, and ch capacity is only one,
				// so if it doesn't go in it means the previous call timed out and hasn't been processed yet.
				// At the same time, the timer is triggered.
				select {
				case ch <- struct{}{}:
				default:
				}
				continue
			}
			if err != nil {
				return err
			}
		case <-ch:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			err := l.Refresh(ctx)
			cancel()
			// timeout here to keep trying
			if errors.Is(err, context.DeadlineExceeded) {
				select {
				case ch <- struct{}{}:
				default:
				}
				continue
			}
			if err != nil {
				return err
			}
		case <-l.unlock:
			return nil
		}
	}
}

func (l *Lock) Unlock(ctx context.Context) error {
	res, err := l.client.Eval(ctx, unlockLua, []string{l.key}, l.value).Int64()
	defer func() {
		// avoiding repeated unlocking to cause a panic
		l.unLockOnce.Do(func() {
			l.unlock <- struct{}{}
			close(l.unlock)
		})
	}()
	if errors.Is(err, redis.Nil) {
		return ErrLockNotHold
	}
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrLockNotHold
	}
	return nil
}
