package common

import (
	"sync"

	"github.com/duanhf2012/origin/v2/log"
)

type Box struct {
	instanceMap map[string]interface{}
	mutex       sync.RWMutex
	// 存储构造函数，用于延迟初始化
	constructorMap map[string]func() interface{}
}

func NewBox() *Box {
	return &Box{
		instanceMap:    make(map[string]interface{}),
		constructorMap: make(map[string]func() interface{}),
	}
}

func (slf *Box) Register(name string, constructor func() interface{}) {
	if constructor == nil {
		log.Error("constructor is nil for: %s", name)
		return
	}

	slf.mutex.Lock()
	defer slf.mutex.Unlock()
	slf.constructorMap[name] = constructor
}

func (slf *Box) RegisterInstance(name string, instance interface{}) {
	slf.mutex.Lock()
	defer slf.mutex.Unlock()
	slf.instanceMap[name] = instance
}

// Get 获取实例，如果不存在则创建
func (slf *Box) Get(name string) interface{} {
	slf.mutex.RLock()
	// 先检查是否已有实例
	if instance, exists := slf.instanceMap[name]; exists {
		slf.mutex.RUnlock()
		return instance
	}
	// 检查是否有构造函数
	_, exists := slf.constructorMap[name]
	if !exists {
		slf.mutex.RUnlock()
		log.Error("no constructor or instance found for: %s", name)
		return nil
	}
	slf.mutex.RUnlock()

	// 升级到写锁后再次检查，保证并发下同名实例仅初始化一次。
	slf.mutex.Lock()
	defer slf.mutex.Unlock()

	if instance, exists := slf.instanceMap[name]; exists {
		return instance
	}

	constructor, exists := slf.constructorMap[name]
	if !exists {
		log.Error("no constructor or instance found for: %s", name)
		return nil
	}

	instance := constructor()
	slf.instanceMap[name] = instance
	return instance
}

func (slf *Box) Exists(name string) bool {
	slf.mutex.RLock()
	defer slf.mutex.RUnlock()
	_, exists := slf.instanceMap[name]
	if exists {
		return true
	}
	_, exists = slf.constructorMap[name]
	return exists
}

func (slf *Box) Remove(name string) {
	slf.mutex.Lock()
	defer slf.mutex.Unlock()
	delete(slf.instanceMap, name)
	delete(slf.constructorMap, name)
}

func (slf *Box) Clear() {
	slf.mutex.Lock()
	defer slf.mutex.Unlock()
	slf.instanceMap = make(map[string]interface{})
	slf.constructorMap = make(map[string]func() interface{})
}

var box *Box
var once sync.Once

func GetBox() *Box {
	once.Do(func() {
		box = NewBox()
	})
	return box
}
