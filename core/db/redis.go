package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"juzi-server/common"
	"juzi-server/config"
	"time"

	"github.com/redis/go-redis/v9"
)

const redisBoxNamePrefix = "redis:"

// RedisClient 定义业务侧常用的 Redis 存取接口。
type RedisClient interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Del(ctx context.Context, keys ...string) (int64, error)
	HSet(ctx context.Context, key string, field string, value interface{}) error
	HGet(ctx context.Context, key string, field string) (string, error)
	SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	GetJSON(ctx context.Context, key string, out interface{}) error
	HSetJSON(ctx context.Context, key string, field string, value interface{}) error
	HGetJSON(ctx context.Context, key string, field string, out interface{}) error
	SetAsync(ctx context.Context, key string, value interface{}, expiration time.Duration) *RedisErrorFuture
	GetAsync(ctx context.Context, key string) *RedisStringFuture
	DelAsync(ctx context.Context, keys ...string) *RedisInt64Future
	HSetAsync(ctx context.Context, key string, field string, value interface{}) *RedisErrorFuture
	HGetAsync(ctx context.Context, key string, field string) *RedisStringFuture
	SetJSONAsync(ctx context.Context, key string, value interface{}, expiration time.Duration) *RedisErrorFuture
	GetJSONAsync(ctx context.Context, key string, out interface{}) *RedisErrorFuture
	HSetJSONAsync(ctx context.Context, key string, field string, value interface{}) *RedisErrorFuture
	HGetJSONAsync(ctx context.Context, key string, field string, out interface{}) *RedisErrorFuture
	Raw() *redis.Client
}

type RedisErrorFuture struct {
	resultChan chan error
}

func (slf *RedisErrorFuture) Get() error {
	return <-slf.resultChan
}

type redisStringResult struct {
	value string
	err   error
}

type RedisStringFuture struct {
	resultChan chan redisStringResult
}

func (slf *RedisStringFuture) Get() (string, error) {
	ret := <-slf.resultChan
	return ret.value, ret.err
}

type redisInt64Result struct {
	value int64
	err   error
}

type RedisInt64Future struct {
	resultChan chan redisInt64Result
}

func (slf *RedisInt64Future) Get() (int64, error) {
	ret := <-slf.resultChan
	return ret.value, ret.err
}

type redisBoxClient struct {
	client *redis.Client
}

func (slf *redisBoxClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	return slf.client.Set(ctx, key, value, expiration).Err()
}

func (slf *redisBoxClient) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", errors.New("redis key is empty")
	}
	return slf.client.Get(ctx, key).Result()
}

func (slf *redisBoxClient) Del(ctx context.Context, keys ...string) (int64, error) {
	if len(keys) == 0 {
		return 0, errors.New("redis keys is empty")
	}
	return slf.client.Del(ctx, keys...).Result()
}

func (slf *redisBoxClient) HSet(ctx context.Context, key string, field string, value interface{}) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	if field == "" {
		return errors.New("redis field is empty")
	}
	return slf.client.HSet(ctx, key, field, value).Err()
}

func (slf *redisBoxClient) HGet(ctx context.Context, key string, field string) (string, error) {
	if key == "" {
		return "", errors.New("redis key is empty")
	}
	if field == "" {
		return "", errors.New("redis field is empty")
	}
	return slf.client.HGet(ctx, key, field).Result()
}

func (slf *redisBoxClient) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return slf.client.Set(ctx, key, data, expiration).Err()
}

func (slf *redisBoxClient) GetJSON(ctx context.Context, key string, out interface{}) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	if out == nil {
		return errors.New("out is nil")
	}
	val, err := slf.client.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), out)
}

func (slf *redisBoxClient) HSetJSON(ctx context.Context, key string, field string, value interface{}) error {
	if key == "" {
		return errors.New("redis key is empty")
	}
	if field == "" {
		return errors.New("redis field is empty")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return slf.client.HSet(ctx, key, field, data).Err()
}

func (slf *redisBoxClient) HGetJSON(ctx context.Context, key string, field string, out interface{}) error {
	if out == nil {
		return errors.New("out is nil")
	}
	val, err := slf.HGet(ctx, key, field)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(val), out)
}

func (slf *redisBoxClient) SetAsync(ctx context.Context, key string, value interface{}, expiration time.Duration) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.Set(ctx, key, value, expiration)
	}()
	return future
}

func (slf *redisBoxClient) GetAsync(ctx context.Context, key string) *RedisStringFuture {
	future := &RedisStringFuture{resultChan: make(chan redisStringResult, 1)}
	go func() {
		value, err := slf.Get(ctx, key)
		future.resultChan <- redisStringResult{value: value, err: err}
	}()
	return future
}

func (slf *redisBoxClient) DelAsync(ctx context.Context, keys ...string) *RedisInt64Future {
	future := &RedisInt64Future{resultChan: make(chan redisInt64Result, 1)}
	go func() {
		value, err := slf.Del(ctx, keys...)
		future.resultChan <- redisInt64Result{value: value, err: err}
	}()
	return future
}

func (slf *redisBoxClient) HSetAsync(ctx context.Context, key string, field string, value interface{}) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.HSet(ctx, key, field, value)
	}()
	return future
}

func (slf *redisBoxClient) HGetAsync(ctx context.Context, key string, field string) *RedisStringFuture {
	future := &RedisStringFuture{resultChan: make(chan redisStringResult, 1)}
	go func() {
		value, err := slf.HGet(ctx, key, field)
		future.resultChan <- redisStringResult{value: value, err: err}
	}()
	return future
}

func (slf *redisBoxClient) SetJSONAsync(ctx context.Context, key string, value interface{}, expiration time.Duration) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.SetJSON(ctx, key, value, expiration)
	}()
	return future
}

func (slf *redisBoxClient) GetJSONAsync(ctx context.Context, key string, out interface{}) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.GetJSON(ctx, key, out)
	}()
	return future
}

func (slf *redisBoxClient) HSetJSONAsync(ctx context.Context, key string, field string, value interface{}) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.HSetJSON(ctx, key, field, value)
	}()
	return future
}

func (slf *redisBoxClient) HGetJSONAsync(ctx context.Context, key string, field string, out interface{}) *RedisErrorFuture {
	future := &RedisErrorFuture{resultChan: make(chan error, 1)}
	go func() {
		future.resultChan <- slf.HGetJSON(ctx, key, field, out)
	}()
	return future
}

func (slf *redisBoxClient) Raw() *redis.Client {
	return slf.client
}

func redisBoxName(name string) string {
	return redisBoxNamePrefix + name
}

// InitRedisClientsToBox 初始化 Redis 管理器并将配置中的客户端注册到 Box。
func InitRedisClientsToBox(cfgMgr *config.ConfigManager) error {
	if cfgMgr == nil {
		return errors.New("config manager is nil")
	}

	if err := InitRedisByConfigManager(cfgMgr); err != nil {
		return err
	}

	redisItems := cfgMgr.GetRedisServices()
	if len(redisItems) == 0 {
		return errors.New("redis services is empty")
	}

	for name := range redisItems {
		client, err := GetRedisClient(name)
		if err != nil {
			return err
		}
		common.GetBox().RegisterInstance(redisBoxName(name), &redisBoxClient{client: client})
	}

	return nil
}

// GetRedisClientFromBox 从 Box 按名称获取 Redis 客户端。
func GetRedisClientFromBox(name string) (RedisClient, error) {
	if name == "" {
		return nil, errors.New("redis client name is empty")
	}

	instance := common.GetBox().Get(redisBoxName(name))
	if instance == nil {
		return nil, fmt.Errorf("redis client %s not found in box", name)
	}

	client, ok := instance.(RedisClient)
	if !ok {
		return nil, fmt.Errorf("redis client %s type assertion failed", name)
	}

	return client, nil
}
