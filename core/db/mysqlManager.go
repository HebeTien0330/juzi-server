package db

import (
	"errors"
	"fmt"
	"juzi-server/config"
	"pb"
	"sync"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type MysqlOptions struct {
	MaxIdleConns    int
	MaxOpenConns    int
	ConnMaxLifetime time.Duration
	SlowThreshold   time.Duration
	LogLevel        logger.LogLevel
}

type mysqlManager struct {
	mu      sync.RWMutex
	clients map[string]*gorm.DB
	options MysqlOptions
}

var defaultMysqlManager = newMysqlManager()

func newMysqlManager() *mysqlManager {
	return &mysqlManager{
		clients: make(map[string]*gorm.DB),
		options: defaultMysqlOptions(),
	}
}

func defaultMysqlOptions() MysqlOptions {
	return MysqlOptions{
		MaxIdleConns:    10,
		MaxOpenConns:    50,
		ConnMaxLifetime: 90 * time.Second,
		SlowThreshold:   500 * time.Millisecond,
		LogLevel:        logger.Warn,
	}
}

func SetMysqlOptions(options MysqlOptions) {
	defaultMysqlManager.SetOptions(options)
}

func (m *mysqlManager) SetOptions(options MysqlOptions) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if options.MaxIdleConns <= 0 {
		options.MaxIdleConns = 10
	}
	if options.MaxOpenConns <= 0 {
		options.MaxOpenConns = 50
	}
	if options.ConnMaxLifetime <= 0 {
		options.ConnMaxLifetime = 90 * time.Second
	}
	if options.SlowThreshold <= 0 {
		options.SlowThreshold = 500 * time.Millisecond
	}
	if options.LogLevel < logger.Silent || options.LogLevel > logger.Info {
		options.LogLevel = logger.Warn
	}

	m.options = options
}

func InitMysqlByConfigManager(cfgMgr *config.ConfigManager) error {
	if cfgMgr == nil {
		return errors.New("config manager is nil")
	}
	return InitMysqlByItems(cfgMgr.GetMySQLServices())
}

func InitMysqlByItems(items map[string]*pb.MySqlItem) error {
	return defaultMysqlManager.InitByItems(items)
}

func (m *mysqlManager) InitByItems(items map[string]*pb.MySqlItem) error {
	if len(items) == 0 {
		return errors.New("mysql items is empty")
	}

	for name, item := range items {
		if err := m.AddClient(name, item); err != nil {
			return err
		}
	}

	return nil
}

func AddMysqlClient(name string, item *pb.MySqlItem) error {
	return defaultMysqlManager.AddClient(name, item)
}

func (m *mysqlManager) AddClient(name string, item *pb.MySqlItem) error {
	if name == "" {
		return errors.New("mysql client name is empty")
	}
	if item == nil {
		return fmt.Errorf("mysql item %s is nil", name)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if oldClient, ok := m.clients[name]; ok {
		_ = closeMysqlClient(oldClient)
		delete(m.clients, name)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&readTimeout=30s&timeout=15s&writeTimeout=30s",
		item.GetUser(),
		item.GetPassword(),
		item.GetHost(),
		item.GetPort(),
		item.GetDb(),
	)

	gormCfg := &gorm.Config{
		Logger: logger.Default.LogMode(m.options.LogLevel),
	}

	client, err := gorm.Open(mysql.Open(dsn), gormCfg)
	if err != nil {
		return fmt.Errorf("open mysql client %s failed: %w", name, err)
	}

	sqlDB, err := client.DB()
	if err != nil {
		return fmt.Errorf("get sql db from mysql client %s failed: %w", name, err)
	}

	sqlDB.SetMaxIdleConns(m.options.MaxIdleConns)
	sqlDB.SetMaxOpenConns(m.options.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(m.options.ConnMaxLifetime)

	if err = sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return fmt.Errorf("ping mysql client %s failed: %w", name, err)
	}

	m.clients[name] = client
	return nil
}

func GetMysqlClient(name string) (*gorm.DB, error) {
	return defaultMysqlManager.GetClient(name)
}

func (m *mysqlManager) GetClient(name string) (*gorm.DB, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[name]
	if !ok {
		return nil, fmt.Errorf("mysql client %s not found", name)
	}

	return client, nil
}

func CloseMysqlClient(name string) error {
	return defaultMysqlManager.CloseClient(name)
}

func (m *mysqlManager) CloseClient(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	client, ok := m.clients[name]
	if !ok {
		return nil
	}

	err := closeMysqlClient(client)
	delete(m.clients, name)
	return err
}

func CloseAllMysqlClients() error {
	return defaultMysqlManager.CloseAll()
}

func (m *mysqlManager) CloseAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var retErr error
	for name, client := range m.clients {
		if err := closeMysqlClient(client); err != nil {
			retErr = fmt.Errorf("close mysql client %s failed: %w", name, err)
		}
		delete(m.clients, name)
	}

	return retErr
}

func closeMysqlClient(client *gorm.DB) error {
	if client == nil {
		return nil
	}

	sqlDB, err := client.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
