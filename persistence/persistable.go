package persistence

// Persistable 玩家模块存盘契约：模块自己负责序列化，DB 读写由 ModuleStore 统一处理。
type Persistable interface {
	ModuleName() string
	MarshalPersist() ([]byte, error)
	UnmarshalPersist([]byte) error
}
