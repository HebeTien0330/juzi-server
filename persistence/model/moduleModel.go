package model

import "time"

// PlayerModuleData 通用玩家模块存盘表：一行 = 一个玩家的一个模块。
type PlayerModuleData struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	UserId    string `gorm:"uniqueIndex:idx_user_module;size:64;not null"`
	Module    string `gorm:"uniqueIndex:idx_user_module;size:64;not null"`
	Payload   string `gorm:"type:text;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
