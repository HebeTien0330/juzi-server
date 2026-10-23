package table

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type TableManager struct {
	mu           sync.RWMutex
	basePath     string
	loadedTables map[string]any
}

func NewManager() *TableManager {
	return &TableManager{
		basePath:     "./resources",
		loadedTables: make(map[string]any),
	}
}

var Manager = NewManager()

func (m *TableManager) SetBasePath(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.basePath = path
}

func (m *TableManager) QueryFromLoaded(name string) (any, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	table, ok := m.loadedTables[name]
	return table, ok
}

func (m *TableManager) GetTable(name string, path ...string) (any, error) {
	table, err := m.loadIfNeeded(name)
	if err != nil {
		return nil, err
	}
	if len(path) == 0 || path[0] == "" || path[0] == "*" {
		return table, nil
	}
	result := m.Match(table, path[0])
	if result == nil {
		return nil, fmt.Errorf("table %s path %q not found", name, path[0])
	}
	return result, nil
}

func (m *TableManager) Match(data any, path string) any {
	if path == "" {
		return data
	}
	for _, key := range strings.Split(path, ".") {
		obj, ok := data.(map[string]any)
		if !ok {
			return nil
		}
		val, exists := obj[key]
		if !exists {
			return nil
		}
		data = val
	}
	return data
}

func (m *TableManager) loadIfNeeded(name string) (any, error) {
	if table, ok := m.QueryFromLoaded(name); ok {
		return table, nil
	}
	table, err := m.readTable(name)
	if err != nil {
		return nil, err
	}
	m.appendToLoaded(name, table)
	if cached, ok := m.QueryFromLoaded(name); ok {
		return cached, nil
	}
	return table, nil
}

func (m *TableManager) appendToLoaded(name string, table any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.loadedTables[name]; exists {
		return
	}
	m.loadedTables[name] = table
}

func (m *TableManager) readTable(name string) (any, error) {
	path := filepath.Join(m.basePath, name+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read table %s: %w", name, err)
	}
	var table any
	if err := json.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("parse table %s: %w", name, err)
	}
	return table, nil
}

func Get(name string, path ...string) (any, error) {
	return Manager.GetTable(name, path...)
}
