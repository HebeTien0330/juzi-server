package main

import (
	"juzi-server/config"
	"juzi-server/core/box"
	"juzi-server/core/db"
	"juzi-server/core/table"

	_ "juzi-server/app/battle"
	_ "juzi-server/app/center"
	_ "juzi-server/app/game"
	_ "juzi-server/app/user"
	_ "juzi-server/network"

	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
)

func main() {
	log.OpenConsole = true
	cluster.SetConfigDir("./config")
	box.Add(table.Manager)
	cfg := config.GetConfigManager()
	if err := db.InitMysqlByConfigManager(cfg); err != nil {
		log.SError("init mysql failed: ", err)
		return
	}
	if err := db.AutoMigrate("mysql-1"); err != nil {
		log.SError("auto migrate failed: ", err)
		return
	}
	if err := db.InitRedisClientsToBox(cfg); err != nil {
		log.SError("init redis failed: ", err)
		return
	}
	node.Start()
}
