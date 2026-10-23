package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"pb"

	"github.com/duanhf2012/origin/v2/log"
)

type flatServerConf struct {
	HttpServices      map[string]map[string]*pb.HttpServiceItem      `json:"httpServices"`
	WebSocketServices map[string]map[string]*pb.WebSocketServiceItem `json:"websocketServices"`
	RedisService      map[string]*pb.RedisItem                       `json:"redisService"`
	MysqlService      map[string]*pb.MySqlItem                       `json:"mysqlService"`
	Version           string                                         `json:"version"`
}

func GetConfigManager() *ConfigManager {
	configMgr := &ConfigManager{}
	configMgr.Init()
	return configMgr
}

type ConfigManager struct {
	config *pb.ServerConf
}

func (slf *ConfigManager) Init() error {
	slf.config = slf.loadConfig()
	return nil
}

func (slf *ConfigManager) loadConfig() *pb.ServerConf {
	curDir, _ := os.Getwd()
	configPath := filepath.Join(curDir, "config", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		log.SError("Open config file error:", err)
		return nil
	}

	var rawConfig flatServerConf
	err = json.Unmarshal(data, &rawConfig)
	if err != nil {
		log.SError("Decode config file error:", err)
		return nil
	}

	config := &pb.ServerConf{
		HttpServices:      make(map[string]*pb.HttpServiceGroup, len(rawConfig.HttpServices)),
		WebsocketServices: make(map[string]*pb.WebSocketServiceGroup, len(rawConfig.WebSocketServices)),
		RedisService:      rawConfig.RedisService,
		MysqlService:      rawConfig.MysqlService,
		Version:           rawConfig.Version,
	}

	for serviceName, items := range rawConfig.HttpServices {
		config.HttpServices[serviceName] = &pb.HttpServiceGroup{Items: items}
	}

	for serviceName, items := range rawConfig.WebSocketServices {
		config.WebsocketServices[serviceName] = &pb.WebSocketServiceGroup{Items: items}
	}

	return config
}

// HTTP 服务相关
func (slf *ConfigManager) GetHttpServices() map[string]*pb.HttpServiceGroup {
	if slf.config == nil {
		return nil
	}
	return slf.config.HttpServices
}

func (slf *ConfigManager) GetHttpServiceGroup(serviceName string) *pb.HttpServiceGroup {
	if slf.config == nil {
		return nil
	}
	return slf.config.HttpServices[serviceName]
}

func (slf *ConfigManager) GetHttpServiceItem(serviceName string, itemId string) *pb.HttpServiceItem {
	if slf.config == nil {
		return nil
	}
	group := slf.config.HttpServices[serviceName]
	if group == nil {
		return nil
	}
	return group.Items[itemId]
}

// WebSocket 服务相关
func (slf *ConfigManager) GetWebSocketServices() map[string]*pb.WebSocketServiceGroup {
	if slf.config == nil {
		return nil
	}
	return slf.config.WebsocketServices
}

func (slf *ConfigManager) GetWebSocketServiceGroup(serviceName string) *pb.WebSocketServiceGroup {
	if slf.config == nil {
		return nil
	}
	return slf.config.WebsocketServices[serviceName]
}

func (slf *ConfigManager) GetWebSocketServiceItem(serviceName string, itemId string) *pb.WebSocketServiceItem {
	if slf.config == nil {
		return nil
	}
	group := slf.config.WebsocketServices[serviceName]
	if group == nil {
		return nil
	}
	return group.Items[itemId]
}

// Redis 服务相关（直接是 map，无 Group 包装）
func (slf *ConfigManager) GetRedisServices() map[string]*pb.RedisItem {
	if slf.config == nil {
		return nil
	}
	return slf.config.RedisService
}

func (slf *ConfigManager) GetRedisServiceItem(serviceName string) *pb.RedisItem {
	if slf.config == nil {
		return nil
	}
	return slf.config.RedisService[serviceName]
}

// MySQL 服务相关（直接是 map，无 Group 包装）
func (slf *ConfigManager) GetMySQLServices() map[string]*pb.MySqlItem {
	if slf.config == nil {
		return nil
	}
	return slf.config.MysqlService
}

func (slf *ConfigManager) GetMySQLServiceItem(serviceName string) *pb.MySqlItem {
	if slf.config == nil {
		return nil
	}
	return slf.config.MysqlService[serviceName]
}

// 版本信息
func (slf *ConfigManager) GetVersion() string {
	if slf.config == nil {
		return ""
	}
	return slf.config.Version
}
