package app

import (
	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
)

func init() {
	log.Info("setup user server")
	node.Setup(&User{})
}

type User struct {
	service.Service
}

func (slf *User) OnInit() error {
	log.Info("user server start ok!")
	return nil
}

func (slf *User) GetName() string {
	return "User"
}

func (slf *User) GetServerId() string {
	return cluster.GetCluster().GetLocalNodeInfo().NodeId
}
