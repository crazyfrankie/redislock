package redislock

import "time"

type RetryStrategy interface {
	// Next returns the interval for the next retry,
	// if no further retries are needed, then the second parameter returns false.
	Next() (time.Duration, bool)
}

type FixIntervalRetry struct {
	Interval time.Duration // retry interval
	Max      int           // max retry times
	cnt      int           // retry count
}

func (f *FixIntervalRetry) Next() (time.Duration, bool) {
	f.cnt++
	return f.Interval, f.cnt <= f.Max
}
