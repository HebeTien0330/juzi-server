package game

import (
	"errors"

	"juzi-server/config"
	"juzi-server/core/actor"
	"juzi-server/core/box"
	"juzi-server/core/protocol"
	"juzi-server/module"
	"pb"

	"github.com/duanhf2012/origin/v2/cluster"
	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/network/processor"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func init() {
	log.Info("setup game server")
	node.Setup(&Game{})
}

type Game struct {
	service.Service
	processor    *processor.PBProcessor
	router       *protocol.Router
	centerServId string
	graceTimers  map[string]*uint64
}

func (slf *Game) OnInit() error {
	slf.OpenConcurrentByNumCPU(1.0)
	slf.processor = processor.NewPBProcessor()
	slf.router = protocol.NewRouter()
	slf.graceTimers = make(map[string]*uint64)
	slf.centerServId = slf.lookupCenterServerId()
	box.Add(actor.NewManager())
	slf.registerSessionRoutes()
	module.Install(slf.router)
	log.Info("game server start ok, serverId=", slf.GetServerId())
	return nil
}

func (slf *Game) GetName() string {
	return "Game"
}

func (slf *Game) GetServerId() string {
	return cluster.GetCluster().GetLocalNodeInfo().NodeId
}

func (slf *Game) Router() *protocol.Router {
	return slf.router
}

func (slf *Game) ActorManager() *actor.Manager {
	manager := box.Get((*actor.Manager)(nil))
	if manager == nil {
		return nil
	}
	return manager.(*actor.Manager)
}

// CompleteLogin 创建在线对象、签发 token，并通知 Gate 绑定连接。
func (slf *Game) CompleteLogin(clientId, userId, account string) (*actor.Actor, string, error) {
	manager := slf.ActorManager()
	if manager == nil {
		return nil, "", errors.New("actor manager is nil")
	}
	a, token, err := manager.Enter(userId, account, clientId)
	if err != nil {
		return nil, "", err
	}
	req := &pb.BindSessionReq{
		BaseReq:      &pb.InnerBaseReq{ServerId: slf.GetServerId()},
		ClientId:     clientId,
		UserId:       userId,
		Account:      account,
		GameServerId: slf.GetServerId(),
	}
	rsp := &pb.BindSessionRsp{}
	if err := slf.CallNode(slf.gateNodeId(), "Gate.RPC_BindSession", req, rsp); err != nil {
		manager.Leave(userId)
		return nil, "", err
	}
	if rsp.GetBaseRsp().GetCode() != pb.Code_OK {
		manager.Leave(userId)
		return nil, "", errors.New("bind session rejected")
	}
	return a, token, nil
}

func (slf *Game) Notify(clientIds []string, msg proto.Message) {
	if msg == nil {
		return
	}
	msgName := string(msg.ProtoReflect().Descriptor().Name())
	payload, err := proto.Marshal(msg)
	if err != nil {
		log.SError("marshal notify message error ", err)
		return
	}
	for _, clientId := range clientIds {
		if clientId == "" {
			continue
		}
		err := slf.CastGo("Gate.RPC_ReceiveNotifyMessage", &pb.NotifyMessageInf{
			BaseReq: &pb.BaseReq{ClientId: clientId},
			MsgName: msgName,
			Payload: payload,
		})
		if err != nil {
			log.SError("cast notify message to gate error ", err)
		}
	}
}

func (slf *Game) RpcCall(path string, req proto.Message, callback func(rsp proto.Message, err error)) error {
	payload, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	rpcReq := &pb.RpcMessageReq{
		BaseReq: &pb.InnerBaseReq{ServerId: slf.GetServerId()},
		MsgName: path,
		Payload: payload,
	}
	return slf.AsyncCallNode(slf.centerServerId(), "Center.RPC_ReceiveRemoteMessage", rpcReq, func(rpcRsp *pb.RpcMessageRsp, err error) {
		rspMsg, parseErr := parseCenterRpcResponse(rpcRsp, err)
		if callback != nil {
			callback(rspMsg, parseErr)
		}
	})
}

func parseCenterRpcResponse(rpcRsp *pb.RpcMessageRsp, err error) (proto.Message, error) {
	if err != nil {
		return nil, err
	}
	if base := rpcRsp.GetBaseRsp(); base != nil && base.GetCode() != pb.Code_OK {
		msg := rpcRsp.GetMsg()
		if msg == "" {
			msg = "rpc call failed"
		}
		return nil, errors.New(msg)
	}
	if rpcRsp.GetMsgName() == "" {
		return nil, nil
	}
	msgType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(rpcRsp.GetMsgName()))
	if err != nil {
		return nil, err
	}
	rspMsg := msgType.New().Interface()
	if err := proto.Unmarshal(rpcRsp.GetPayload(), rspMsg); err != nil {
		return nil, err
	}
	return rspMsg, nil
}

func (slf *Game) centerServerId() string {
	if slf.centerServId == "" {
		slf.centerServId = slf.lookupCenterServerId()
	}
	return slf.centerServId
}

func (slf *Game) lookupCenterServerId() string {
	cfg := config.GetConfigManager()
	cur := slf.GetServerId()
	group := cfg.GetWebSocketServiceGroup("game")
	if group == nil {
		return ""
	}
	var master string
	for _, item := range group.Items {
		if item != nil && item.No == cur {
			master = item.Master
			break
		}
	}
	if master == "" {
		return ""
	}
	centerGroup := cfg.GetWebSocketServiceGroup("center")
	if centerGroup != nil {
		for _, item := range centerGroup.Items {
			if item != nil && (item.No == master || item.Id == master) {
				return item.No
			}
		}
	}
	return master
}

func (slf *Game) gateNodeId() string {
	group := config.GetConfigManager().GetWebSocketServiceGroup("gate")
	if group == nil {
		return ""
	}
	for _, item := range group.Items {
		if item != nil && item.GetNo() != "" {
			return item.GetNo()
		}
	}
	return ""
}
