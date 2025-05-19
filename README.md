# RedisLock
基于 Redis 实现的分布式锁

## Features
### v1
- [SingleflightLock](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L50-L70)
- [Lock(With Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L72-L113)
- [TryLock(No Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L115-L131)
- [AutoRefresh](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L163-L209)
- [Unlock](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L211-L230)
- [SetValuer (Set user's own valuer function)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L44-L46)