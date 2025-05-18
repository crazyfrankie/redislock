package redislock

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/crazyfrankie/redislock/mocks"
)

func TestClient_Lock(t *testing.T) {
	t.Parallel()
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	testCases := []struct {
		name string

		mock func() redis.Cmdable

		key        string
		expiration time.Duration
		retry      RetryStrategy
		timeout    time.Duration

		wantLock *Lock
		wantErr  string
	}{
		{
			name: "locked",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolCmd(context.Background())
				res.SetVal(true)
				cmdable.EXPECT().SetNX(gomock.Any(), "locked-key", gomock.Any(), time.Minute).
					Return(res)
				return cmdable
			},
			key:        "locked-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Second, Max: 1},
			timeout:    time.Second,
			wantLock: &Lock{
				key:        "locked-key",
				expiration: time.Minute,
			},
		},
		{
			name: "not retryable",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolCmd(context.Background())
				res.SetErr(errors.New("network error"))
				cmdable.EXPECT().SetNX(gomock.Any(), "locked-key", gomock.Any(), time.Minute).
					Return(res)
				return cmdable
			},
			key:        "locked-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Second, Max: 1},
			timeout:    time.Second,
			wantErr:    "network error",
		},
		{
			name: "retry over times",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolCmd(context.Background())
				res.SetErr(context.DeadlineExceeded)
				cmdable.EXPECT().SetNX(gomock.Any(), "retry-key", gomock.Any(), time.Minute).
					Times(3).Return(res)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, Max: 2},
			timeout:    time.Second,
			wantErr:    "retry lock: retries are exhausted, last retry failed: context deadline exceeded",
		},
		{
			name: "retry over times-lock holded",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolCmd(context.Background())
				res.SetVal(false)
				cmdable.EXPECT().SetNX(gomock.Any(), "retry-key", gomock.Any(), time.Minute).
					Times(3).Return(res)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, Max: 2},
			timeout:    time.Second,
			wantErr:    fmt.Sprintf("retry lock: retries are exhausted, the lock is being held: %s", ErrFailedToPreemptLock),
		},
		{
			name: "retry and success",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				failRes := redis.NewBoolCmd(context.Background())
				failRes.SetVal(false)
				cmdable.EXPECT().SetNX(gomock.Any(), "retry-key", gomock.Any(), time.Minute).
					Times(2).Return(failRes)

				successRes := redis.NewBoolCmd(context.Background())
				successRes.SetVal(true)
				cmdable.EXPECT().SetNX(gomock.Any(), "retry-key", gomock.Any(), time.Minute).
					Return(successRes)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond, Max: 3},
			timeout:    time.Second,
			wantLock: &Lock{
				key:        "retry-key",
				expiration: time.Minute,
			},
		},
		{
			name: "retry but timeout",
			mock: func() redis.Cmdable {
				cmdable := mocks.NewMockCmdable(ctrl)
				res := redis.NewBoolCmd(context.Background())
				res.SetVal(false)
				cmdable.EXPECT().SetNX(gomock.Any(), "retry-key", gomock.Any(), time.Minute).
					Times(2).Return(res)
				return cmdable
			},
			key:        "retry-key",
			expiration: time.Minute,
			retry:      &FixIntervalRetry{Interval: time.Millisecond * 550, Max: 2},
			timeout:    time.Second,
			wantErr:    "context deadline exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockRedisCmd := tc.mock()
			client := NewClient(mockRedisCmd)
			// Set a fixed value for testing
			client.SetValuer(func() string {
				return "test-value"
			})
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			l, err := client.Lock(ctx, tc.key, tc.expiration, tc.timeout, tc.retry)
			if tc.wantErr != "" {
				assert.EqualError(t, err, tc.wantErr)
				return
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, mockRedisCmd, l.client)
			assert.Equal(t, tc.key, l.key)
			assert.Equal(t, tc.expiration, l.expiration)
			assert.Equal(t, "test-value", l.value)
		})
	}
}
