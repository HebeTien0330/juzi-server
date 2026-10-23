package model

import "time"

// Account 是框架保存的账号身份。玩法数据放在 PlayerModuleData。
type Account struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	UserId    string `gorm:"uniqueIndex;size:64;not null"`
	Account   string `gorm:"uniqueIndex;size:128;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
