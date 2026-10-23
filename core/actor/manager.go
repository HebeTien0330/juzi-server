package actor

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	datasync "juzi-server/core/sync"
	"juzi-server/persistence"
	"juzi-server/persistence/handler"
)

// ReconnectGrace 掉线后保留内存会话的时长。
const ReconnectGrace = 60 * time.Second

// ModuleFactory 在玩家第一次进入本进程时创建需要存盘的模块。
type ModuleFactory func(a *Actor) []persistence.Persistable

type Actor struct {
	UserId       string
	Account      string
	ClientId     string
	Token        string
	Disconnected bool

	modules []persistence.Persistable
	roots   map[string]any
	syncer  *datasync.Synchronizer
	store   *handler.ModuleStore
}

func (a *Actor) Modules() []persistence.Persistable {
	if a == nil {
		return nil
	}
	return a.modules
}

// Publish 登记一个同步根。字段名会转成小驼峰点路径。
func (a *Actor) Publish(root string, value any) {
	if a == nil || root == "" {
		return
	}
	if a.roots == nil {
		a.roots = make(map[string]any)
	}
	a.roots[root] = value
	a.ensureSyncer()
	a.syncer.Diff(root, value)
}

func (a *Actor) FlushRemote() map[string]string {
	if a == nil || a.syncer == nil {
		return nil
	}
	return a.syncer.Take()
}

func (a *Actor) SaveModules() error {
	if a == nil || a.store == nil {
		return nil
	}
	return a.store.SaveAll(a.UserId, a.modules)
}

func (a *Actor) ensureSyncer() {
	if a.syncer == nil {
		a.syncer = datasync.NewSynchronizer()
	}
}

func (a *Actor) issueToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	a.Token = hex.EncodeToString(buf)
	return a.Token, nil
}

type Manager struct {
	mu       sync.Mutex
	byUser   map[string]*Actor
	byClient map[string]string
	factory  ModuleFactory
	store    *handler.ModuleStore
	exportFn func(a *Actor) ([]byte, error)
	importFn func(a *Actor, snapshot []byte) error
}

func NewManager() *Manager {
	store := &handler.ModuleStore{}
	store.Init()
	return &Manager{
		byUser:   make(map[string]*Actor),
		byClient: make(map[string]string),
		store:    store,
	}
}

func (m *Manager) SetModuleFactory(factory ModuleFactory) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.factory = factory
	m.mu.Unlock()
}

func (m *Manager) SetSnapshotHooks(exportFn func(a *Actor) ([]byte, error), importFn func(a *Actor, snapshot []byte) error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.exportFn = exportFn
	m.importFn = importFn
	m.mu.Unlock()
}

// Enter 建立或恢复在线对象，并签发新的 sessionToken。
func (m *Manager) Enter(userId, account, clientId string) (*Actor, string, error) {
	if userId == "" {
		return nil, "", errors.New("userId is empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.byUser[userId]
	if !ok {
		a = &Actor{
			UserId:  userId,
			Account: account,
			roots:   make(map[string]any),
			syncer:  datasync.NewSynchronizer(),
			store:   m.store,
		}
		if m.factory != nil {
			a.modules = m.factory(a)
		}
		if err := m.store.LoadAll(userId, a.modules); err != nil {
			return nil, "", err
		}
		m.byUser[userId] = a
	}
	if account != "" {
		a.Account = account
	}
	m.rebindLocked(a, clientId)
	a.Disconnected = false
	token, err := a.issueToken()
	if err != nil {
		return nil, "", err
	}
	return a, token, nil
}

func (m *Manager) Reconnect(userId, clientId, token string) (*Actor, string, error) {
	if userId == "" || token == "" {
		return nil, "", errors.New("reconnect session is empty")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.byUser[userId]
	if a == nil || a.Token == "" || a.Token != token {
		return nil, "", errors.New("session expired")
	}
	m.rebindLocked(a, clientId)
	a.Disconnected = false
	next, err := a.issueToken()
	if err != nil {
		return nil, "", err
	}
	return a, next, nil
}

func (m *Manager) MarkDisconnected(userId string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.byUser[userId]
	if a == nil {
		return nil
	}
	a.Disconnected = true
	if a.ClientId != "" {
		delete(m.byClient, a.ClientId)
		a.ClientId = ""
	}
	return a
}

func (m *Manager) Leave(userId string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	a := m.byUser[userId]
	if a == nil {
		return nil
	}
	if a.ClientId != "" {
		delete(m.byClient, a.ClientId)
	}
	delete(m.byUser, userId)
	return a
}

func (m *Manager) Get(userId string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.byUser[userId]
}

func (m *Manager) GetByClient(clientId string) *Actor {
	m.mu.Lock()
	defer m.mu.Unlock()
	userId := m.byClient[clientId]
	if userId == "" {
		return nil
	}
	return m.byUser[userId]
}

func (m *Manager) Export(userId string) (*Actor, []byte, error) {
	m.mu.Lock()
	a := m.byUser[userId]
	exportFn := m.exportFn
	m.mu.Unlock()
	if a == nil {
		return nil, nil, errors.New("player not found")
	}
	var snapshot []byte
	var err error
	if exportFn != nil {
		snapshot, err = exportFn(a)
		if err != nil {
			return nil, nil, err
		}
	}
	m.Leave(userId)
	return a, snapshot, nil
}

func (m *Manager) Import(userId, account, clientId, token string, snapshot []byte) (*Actor, error) {
	if userId == "" {
		return nil, errors.New("userId is empty")
	}
	m.mu.Lock()
	importFn := m.importFn
	a := m.byUser[userId]
	if a == nil {
		a = &Actor{
			UserId:  userId,
			Account: account,
			Token:   token,
			roots:   make(map[string]any),
			syncer:  datasync.NewSynchronizer(),
			store:   m.store,
		}
		if m.factory != nil {
			a.modules = m.factory(a)
		}
		m.byUser[userId] = a
	}
	if account != "" {
		a.Account = account
	}
	if token != "" {
		a.Token = token
	}
	m.rebindLocked(a, clientId)
	a.Disconnected = false
	m.mu.Unlock()

	if len(a.modules) > 0 {
		if err := a.store.LoadAll(userId, a.modules); err != nil {
			return nil, err
		}
	}
	if importFn != nil && len(snapshot) > 0 {
		if err := importFn(a, snapshot); err != nil {
			return nil, err
		}
	}
	return a, nil
}

func (m *Manager) rebindLocked(a *Actor, clientId string) {
	if a.ClientId != "" && a.ClientId != clientId {
		delete(m.byClient, a.ClientId)
	}
	a.ClientId = clientId
	if clientId != "" {
		m.byClient[clientId] = a.UserId
	}
}
