# RedisLock
基于 Redis 实现的分布式锁

## Features
### v1
- [Lock(With Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L72-L113)
- [TryLock(No Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L115-L131)
- [AutoRefresh](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L163-L209)
- [Unlock](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L211-L230)
- [SetValuer (Set user's own valuer function)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L44-L46)
### v2
- [SingleflightLock](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L50-L70)

## Notice
本项目的实现仅学习使用：只适用于单 Redis 实例的场景，且小规模、业务量不大的场景
