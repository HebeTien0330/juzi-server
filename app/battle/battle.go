package app

import (
	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
)

func init() {
	log.Info("setup battle server")
	node.Setup(&Battle{})
}

type Battle struct {
	service.Service
}

func (slf *Battle) OnInit() error {
	log.Info("battle server start ok!")
	return nil
}

func (slf *Battle) GetName() string {
	return "Battle"
}

func (slf *Battle) GetServerId() string {
	return cluster.GetCluster().GetLocalNodeInfo().NodeId
}
