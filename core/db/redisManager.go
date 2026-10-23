package db

import (
	"context"
	"errors"
	"fmt"
	"juzi-server/config"
	"pb"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisOptions struct {
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type redisManager struct {
	mu      sync.RWMutex
	clients map[string]*redis.Client
	options RedisOptions
}

var defaultRedisManager = newRedisManager()

func newRedisManager() *redisManager {
	return &redisManager{
		clients: make(map[string]*redis.Client),
		options: defaultRedisOptions(),
	}
}

func defaultRedisOptions() RedisOptions {
	return RedisOptions{
		PoolSize:     20,
		MinIdleConns: 5,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

func SetRedisOptions(options RedisOptions) {
	defaultRedisManager.SetOptions(options)
}

func (m *redisManager) SetOptions(options RedisOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if options.PoolSize <= 0 {
		options.PoolSize = 20
	}
	if options.MinIdleConns < 0 {
		options.MinIdleConns = 0
	}
	if options.DialTimeout <= 0 {
		options.DialTimeout = 5 * time.Second
	}
	if options.ReadTimeout <= 0 {
		options.ReadTimeout = 3 * time.Second
	}
	if options.WriteTimeout <= 0 {
		options.WriteTimeout = 3 * time.Second
	}

	m.options = options
}

func InitRedisByConfigManager(cfgMgr *config.ConfigManager) error {
	if cfgMgr == nil {
		return errors.New("config manager is nil")
	}
	return InitRedisByItems(cfgMgr.GetRedisServices())
}

func InitRedisByItems(items map[string]*pb.RedisItem) error {
	return defaultRedisManager.InitByItems(items)
}

func (m *redisManager) InitByItems(items map[string]*pb.RedisItem) error {
	if len(items) == 0 {
		return errors.New("redis items is empty")
	}

	for name, item := range items {
		if err := m.AddClient(name, item); err != nil {
			return err
		}
	}

	return nil
}

func AddRedisClient(name string, item *pb.RedisItem) error {
	return defaultRedisManager.AddClient(name, item)
}

func (m *redisManager) AddClient(name string, item *pb.RedisItem) error {
	if name == "" {
		return errors.New("redis client name is empty")
	}
	if item == nil {
		return fmt.Errorf("redis item %s is nil", name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if oldClient, ok := m.clients[name]; ok {
		_ = oldClient.Close()
		delete(m.clients, name)
	}

	client := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%d", item.GetHost(), item.GetPort()),
		Password:     item.GetPassword(),
		DB:           int(item.GetDb()),
		PoolSize:     m.options.PoolSize,
		MinIdleConns: m.options.MinIdleConns,
		DialTimeout:  m.options.DialTimeout,
		ReadTimeout:  m.options.ReadTimeout,
		WriteTimeout: m.options.WriteTimeout,
	})

	ctx, cancel := context.WithTimeout(context.Background(), m.options.DialTimeout)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return fmt.Errorf("ping redis client %s failed: %w", name, err)
	}

	m.clients[name] = client
	return nil
}

func GetRedisClient(name string) (*redis.Client, error) {
	return defaultRedisManager.GetClient(name)
}

func (m *redisManager) GetClient(name string) (*redis.Client, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("redis client %s not found", name)
	}

	return client, nil
}

func CloseRedisClient(name string) error {
	return defaultRedisManager.CloseClient(name)
}

func (m *redisManager) CloseClient(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[name]
	if !ok {
		return nil
	}

	err := client.Close()
	delete(m.clients, name)
	return err
}

func CloseAllRedisClients() error {
	return defaultRedisManager.CloseAll()
}

func (m *redisManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var retErr error
	for name, client := range m.clients {
		if err := client.Close(); err != nil {
			retErr = fmt.Errorf("close redis client %s failed: %w", name, err)
		}
		delete(m.clients, name)
	}

	return retErr
}
