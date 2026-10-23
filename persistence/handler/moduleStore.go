package handler

import (
	"errors"
	"juzi-server/core/db"
	"juzi-server/persistence"
	"juzi-server/persistence/model"
	"time"

	"gorm.io/gorm"
)

type ModuleStore struct {
	dbName string
}

func (slf *ModuleStore) Init() {
	slf.dbName = "mysql-1"
}

func (slf *ModuleStore) Save(userId string, m persistence.Persistable) error {
	if userId == "" {
		return errors.New("user id is empty")
	}
	if m == nil {
		return errors.New("persistable is nil")
	}
	module := m.ModuleName()
	if module == "" {
		return errors.New("module name is empty")
	}

	payload, err := m.MarshalPersist()
	if err != nil {
		return err
	}

	existing, err := db.FindOne[model.PlayerModuleData](slf.dbName, "user_id = ? AND module = ?", userId, module)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return db.Create(slf.dbName, &model.PlayerModuleData{
				UserId:    userId,
				Module:    module,
				Payload:   string(payload),
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			})
		}
		return err
	}

	existing.Payload = string(payload)
	existing.UpdatedAt = time.Now()
	return db.Save(slf.dbName, existing)
}

func (slf *ModuleStore) Load(userId string, m persistence.Persistable) error {
	if userId == "" {
		return errors.New("user id is empty")
	}
	if m == nil {
		return errors.New("persistable is nil")
	}
	module := m.ModuleName()
	if module == "" {
		return errors.New("module name is empty")
	}

	record, err := db.FindOne[model.PlayerModuleData](slf.dbName, "user_id = ? AND module = ?", userId, module)
	if err != nil {
		return err
	}
	if record.Payload == "" {
		return m.UnmarshalPersist(nil)
	}
	return m.UnmarshalPersist([]byte(record.Payload))
}

func (slf *ModuleStore) SaveAll(userId string, modules []persistence.Persistable) error {
	for _, m := range modules {
		if err := slf.Save(userId, m); err != nil {
			return err
		}
	}
	return nil
}

func (slf *ModuleStore) LoadAll(userId string, modules []persistence.Persistable) error {
	for _, m := range modules {
		err := slf.Load(userId, m)
		if err == nil {
			continue
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		return err
	}
	return nil
}
