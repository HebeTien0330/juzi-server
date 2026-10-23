package center

import (
	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
)

func init() {
	log.Info("setup center server")
	node.Setup(&Center{})
}

type Center struct {
	service.Service
	services map[string]any
}

var pendingServices []namedService

type namedService struct {
	name    string
	service any
}

// Register 在 init 中挂上游戏自己的跨服服务。方法签名为 Method(req proto.Message) (proto.Message, error)。
func Register(name string, service any) {
	if name == "" || service == nil {
		return
	}
	pendingServices = append(pendingServices, namedService{name: name, service: service})
}

func (slf *Center) OnInit() error {
	slf.services = make(map[string]any)
	for _, item := range pendingServices {
		slf.services[item.name] = item.service
	}
	log.Info("center server start ok!")
	return nil
}

func (slf *Center) GetName() string {
	return "Center"
}

func (slf *Center) GetServerId() string {
	return cluster.GetCluster().GetLocalNodeInfo().NodeId
}
