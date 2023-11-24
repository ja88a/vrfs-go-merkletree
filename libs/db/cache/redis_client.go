package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/ja88a/vrfs-go-merkletree/libs/config"
	"github.com/ja88a/vrfs-go-merkletree/libs/logger"
)

// Client structure for redis-base data caching
type CacheClient struct {
	rdb *redis.Client
	ctx context.Context
	log *logger.Logger
}

// Init a new Redis client for caching DB entries
func NewCacheClient(ctx context.Context, l *logger.Logger, cfg *config.Config) *CacheClient {
	endpoint := cfg.Cache.Endpoint
	user := cfg.Cache.User
	password := cfg.Cache.Password
	redisClient := redis.NewClient(&redis.Options{
		Addr:     endpoint,
		Username: user,
		Password: password,
		DB:       0, // use default DB
		Protocol: 3,
	})

	return &CacheClient{
		rdb: redisClient,
		ctx: ctx,
		log: l,
	}
}

func (cc *CacheClient) SetContext(newCtx context.Context) {
	cc.ctx = newCtx
}

func (cc *CacheClient) Get(key string) (interface{}, error) {
	val, err := cc.rdb.Get(cc.ctx, key).Result()
	if err == redis.Nil {
		cc.log.Debug("Cache entry '%v' does not exist", key)
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to retrieve cache entry '%v'\n%w", key, err)
	} else {
		return val, nil
	}
}

func (cc *CacheClient) GetString(key string) (string, error) {
	val, err := cc.Get(key)
	if err != nil {
		return "", err
	}
	if val == nil {
		return "", nil
	}
	valS, ok := val.(string)
	if !ok {
		return valS, fmt.Errorf("failed to retrieve cache entry '%v' - not a string: %v", key, val)
	}
	return valS, nil
}

func (cc *CacheClient) Set(key string, val interface{}, expire time.Duration) error {
	err := cc.rdb.Set(cc.ctx, key, val, expire).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache entry key: %v\n%w", key, err)
	}
	return nil
}
