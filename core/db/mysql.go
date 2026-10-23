package db

import (
	"juzi-server/persistence/model"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CREATE DATABASE IF NOT EXISTS gamedata CHARACTER SET utf8 COLLATE utf8_general_ci;
// AutoMigrate 建表/同步所有表结构（服务启动时调用）
func AutoMigrate(dbName string) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}

	if err := client.AutoMigrate(&model.Account{}, &model.PlayerModuleData{}); err != nil {
		if isIgnorableAutoMigrateError(err) {
			return nil
		}
		return err
	}
	return nil
}

func isIgnorableAutoMigrateError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate column name") ||
		strings.Contains(msg, "Duplicate key name") ||
		strings.Contains(msg, "already exists")
}

// Create 插入一条记录
func Create[T any](dbName string, record *T) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	return client.Create(record).Error
}

// Save 存在则全量更新，不存在则插入（依赖主键）
func Save[T any](dbName string, record *T) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	return client.Save(record).Error
}

// Upsert 按主键冲突时只更新指定列（updateColumns 为空则更新所有列）
func Upsert[T any](dbName string, record *T, updateColumns ...string) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	if len(updateColumns) == 0 {
		return client.Clauses(clause.OnConflict{UpdateAll: true}).Create(record).Error
	}
	cols := make([]clause.Column, 0, len(updateColumns))
	for _, c := range updateColumns {
		cols = append(cols, clause.Column{Name: c})
	}
	return client.Clauses(clause.OnConflict{
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(record).Error
}

// FindByID 按主键查询
func FindByID[T any](dbName string, id any) (*T, error) {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return nil, err
	}
	var record T
	if err = client.First(&record, id).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindOne 按条件查询单条，query 为条件字符串，args 为参数
func FindOne[T any](dbName string, query string, args ...any) (*T, error) {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return nil, err
	}
	var record T
	if err = client.Where(query, args...).First(&record).Error; err != nil {
		return nil, err
	}
	return &record, nil
}

// FindList 按条件查询多条
func FindList[T any](dbName string, query string, args ...any) ([]T, error) {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return nil, err
	}
	var records []T
	if err = client.Where(query, args...).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

// 条件更新指定字段
func UpdateFields[T any](dbName string, fields map[string]any, query string, args ...any) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	var zero T
	return client.Model(&zero).Where(query, args...).Updates(fields).Error
}

// DeleteByID 按主键删除
func DeleteByID[T any](dbName string, id any) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	var zero T
	return client.Delete(&zero, id).Error
}

// Transaction 事务操作，fn 内执行多步操作，任意步骤返回 error 则全部回滚
func Transaction(dbName string, fn func(tx *gorm.DB) error) error {
	client, err := GetMysqlClient(dbName)
	if err != nil {
		return err
	}
	return client.Transaction(fn)
}
