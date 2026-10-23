package gate

import (
	"errors"
	"time"

	"pb"

	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
)

func init() {
	log.SInfo("setup gate server")
	node.Setup(&Gate{})
}

// ReconnectGrace 与 Game 侧一致，掉线后保留路由。
const ReconnectGrace = 60 * time.Second

type NotifyPusher func(clientId string, msgName string, payload []byte) error

type Gate struct {
	service.Service
	players      map[string]string
	clients      map[string]*pb.ClientInfo
	accounts     map[string]string
	userAccount  map[string]string
	provisional  map[string]string
	graceTimers  map[string]*uint64
	notifyPusher NotifyPusher
}

func (slf *Gate) SetNotifyPusher(pusher NotifyPusher) {
	slf.notifyPusher = pusher
}

func (slf *Gate) OnInit() error {
	slf.players = make(map[string]string)
	slf.clients = make(map[string]*pb.ClientInfo)
	slf.accounts = make(map[string]string)
	slf.userAccount = make(map[string]string)
	slf.provisional = make(map[string]string)
	slf.graceTimers = make(map[string]*uint64)
	log.SInfo("gate server is start ok!")
	return nil
}

func (slf *Gate) GetName() string {
	return "Gate"
}

func (slf *Gate) GetServerId() string {
	return cluster.GetCluster().GetLocalNodeInfo().NodeId
}

func (slf *Gate) GetUserId(clientId string) string {
	return slf.players[clientId]
}

func (slf *Gate) GetClientInfo(userId string) *pb.ClientInfo {
	return slf.clients[userId]
}

func (slf *Gate) GetGameServerId(userId string) string {
	info := slf.clients[userId]
	if info == nil {
		return ""
	}
	return info.GetGameServerId()
}

func (slf *Gate) RememberServer(clientId, gameServerId string) {
	if clientId == "" || gameServerId == "" {
		return
	}
	slf.provisional[clientId] = gameServerId
}

func (slf *Gate) ProvisionalServer(clientId string) string {
	return slf.provisional[clientId]
}

func (slf *Gate) ForgetClient(clientId string) {
	delete(slf.provisional, clientId)
	delete(slf.players, clientId)
}

// RPC_BindSession 由 Game 在登录成功后调用。
func (slf *Gate) RPC_BindSession(req *pb.BindSessionReq, rsp *pb.BindSessionRsp) error {
	if rsp == nil {
		return errors.New("bind session response is nil")
	}
	rsp.BaseRsp = &pb.InnerBaseRsp{Code: pb.Code_OK}
	if req == nil || req.GetClientId() == "" || req.GetUserId() == "" || req.GetGameServerId() == "" {
		rsp.BaseRsp.Code = pb.Code_ERR
		return errors.New("invalid bind session request")
	}
	slf.bindSession(req.GetAccount(), req.GetUserId(), &pb.ClientInfo{
		ClientId:     req.GetClientId(),
		GameServerId: req.GetGameServerId(),
	})
	delete(slf.provisional, req.GetClientId())
	log.SInfo("bind session user=", req.GetUserId(), " client=", req.GetClientId(), " game=", req.GetGameServerId())
	return nil
}
