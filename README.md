# RedisLock
基于 Redis 实现的分布式锁

## Features
### v1
- [Lock(With Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L48-L89)
- [TryLock(No Retry)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L91-L107)
- [AutoRefresh](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L139-L185)
- [Unlock](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L187-L206)
- [SetValuer (Set user's own valuer function)](https://github.com/crazyfrankie/redislock/blob/master/lock.go/#L44-L46)