package box

import (
	"fmt"
	"reflect"
	"sync"
)

// Box is a lightweight dependency injection container.
type Box struct {
	mu        sync.RWMutex
	instances map[reflect.Type]interface{}
}

var defaultBox = NewBox()

func NewBox() *Box {
	return &Box{instances: make(map[reflect.Type]interface{})}
}

func Add(instance interface{}) {
	defaultBox.Add(instance)
}

func Get(constructor interface{}) interface{} {
	return defaultBox.Get(constructor)
}

func AddByClass(constructor interface{}) {
	defaultBox.AddByClass(constructor)
}

func GetByClass(constructor interface{}) (interface{}, bool) {
	return defaultBox.GetByClass(constructor)
}

func (box *Box) Add(instance interface{}) {
	if instance == nil {
		return
	}
	box.mu.Lock()
	defer box.mu.Unlock()
	value := reflect.ValueOf(instance)
	if !value.IsValid() {
		return
	}
	instanceType := value.Type()
	if instanceType.Kind() == reflect.Ptr {
		if value.IsNil() {
			return
		}
		instanceType = instanceType.Elem()
	}
	box.instances[instanceType] = instance
}

func (box *Box) Get(constructor interface{}) interface{} {
	if constructor == nil {
		return nil
	}
	ctorType := reflect.TypeOf(constructor)
	if ctorType == nil {
		return nil
	}
	if ctorType.Kind() == reflect.Ptr {
		ctorType = ctorType.Elem()
	}
	box.mu.RLock()
	inst, ok := box.instances[ctorType]
	box.mu.RUnlock()
	if !ok {
		box.AddByClass(constructor)
		box.mu.RLock()
		inst, ok = box.instances[ctorType]
		box.mu.RUnlock()
	}
	if !ok {
		return nil
	}
	return inst
}

func (box *Box) AddByClass(constructor interface{}) {
	if constructor == nil {
		return
	}
	ctorType := reflect.TypeOf(constructor)
	if ctorType == nil {
		return
	}
	if ctorType.Kind() == reflect.Ptr {
		ctorType = ctorType.Elem()
	}
	box.mu.Lock()
	defer box.mu.Unlock()
	if _, ok := box.instances[ctorType]; ok {
		return
	}
	newValue := reflect.New(ctorType)
	box.instances[ctorType] = newValue.Interface()
}

func (box *Box) GetByClass(constructor interface{}) (interface{}, bool) {
	if constructor == nil {
		return nil, false
	}
	ctorType := reflect.TypeOf(constructor)
	if ctorType == nil {
		return nil, false
	}
	if ctorType.Kind() == reflect.Ptr {
		ctorType = ctorType.Elem()
	}
	box.mu.RLock()
	defer box.mu.RUnlock()

	inst, ok := box.instances[ctorType]
	if !ok {
		return nil, false
	}
	return inst, true
}

func (box *Box) String() string {
	box.mu.RLock()
	defer box.mu.RUnlock()
	return fmt.Sprintf("container has %d instances", len(box.instances))
}
