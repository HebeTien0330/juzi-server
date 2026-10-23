package network

import (
	"math/rand"

	"juzi-server/app/gate"
	"juzi-server/config"
	"juzi-server/core/protocol"
	"pb"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/network/processor"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
	"github.com/duanhf2012/origin/v2/sysservice/wsservice"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const heartbeatSeqId int32 = 1

type WsGateService struct {
	service.Service
	processor  *processor.PBProcessor
	wsService  *wsservice.WSService
	gate       *gate.Gate
	serverList []*pb.WebSocketServiceItem
}

func init() {
	log.Info("setup gate websocket service")
	node.Setup(&wsservice.WSService{})
	node.Setup(&WsGateService{})
}

func (slf *WsGateService) OnInit() error {
	slf.wsService = node.GetService("WSService").(*wsservice.WSService)
	slf.wsService.SetMessageType(websocket.BinaryMessage)
	slf.processor = processor.NewPBProcessor()
	slf.processor.RegisterUnknownMsg(slf.OnUnknownMsg)
	slf.processor.RegisterDisConnected(slf.OnDisconnected)
	slf.processor.RegisterConnected(slf.OnConnected)
	slf.processor.Register(uint16(pb.MsgType_MsgReq), &pb.TransferMessageReq{}, func(clientId string, msg proto.Message) {
		slf.processTransfer(clientId, msg.(*pb.TransferMessageReq))
	})
	slf.wsService.SetProcessor(slf.processor, slf.GetEventHandler())
	slf.gate = node.GetService("Gate").(*gate.Gate)
	slf.gate.SetNotifyPusher(slf.pushNotify)
	slf.loadGameServers()
	log.Info("gate websocket service start ok")
	return nil
}

func (slf *WsGateService) processTransfer(clientId string, req *pb.TransferMessageReq) {
	if req == nil {
		return
	}
	seqId := req.GetSeqId()
	msgName := req.GetMsgName()
	if msgName == "" {
		slf.sendResponse(clientId, seqId, pb.Code_ERR, "msgName is empty", nil)
		return
	}
	if seqId == heartbeatSeqId && msgName != "C2SHeartbeat" {
		slf.sendResponse(clientId, seqId, pb.Code_ERR, "seqId 1 is reserved for heartbeat", nil)
		return
	}
	switch msgName {
	case "C2SHeartbeat":
		slf.handleHeartbeat(clientId, seqId, req.GetPayload())
		return
	case "C2SReconnect":
		reconnectReq := &pb.C2SReconnect{}
		if err := proto.Unmarshal(req.GetPayload(), reconnectReq); err != nil {
			slf.sendResponse(clientId, seqId, pb.Code_ERR, err.Error(), nil)
			return
		}
		err := slf.gate.OnReconnect(clientId, reconnectReq.GetUserId(), req, func(rsp *pb.S2CReconnect, err error) {
			if err != nil {
				slf.sendResponse(clientId, seqId, pb.Code_ERR, err.Error(), nil)
				return
			}
			rsp.Clientid = clientId
			payload, marshalErr := proto.Marshal(rsp)
			if marshalErr != nil {
				slf.sendResponse(clientId, seqId, pb.Code_ERR, marshalErr.Error(), nil)
				return
			}
			code := pb.Code_OK
			msg := ""
			if rsp.GetCode() != pb.Code_OK {
				code = rsp.GetCode()
				msg = "reconnect rejected"
			}
			slf.sendResponse(clientId, seqId, code, msg, payload)
		})
		if err != nil {
			slf.sendResponse(clientId, seqId, pb.Code_ERR, err.Error(), nil)
		}
		return
	}

	serverId := slf.gate.GetGameServerId(slf.gate.GetUserId(clientId))
	if serverId == "" {
		if !protocol.IsAnonymous(msgName) {
			slf.sendResponse(clientId, seqId, pb.Code_ERR, "player is not login", nil)
			return
		}
		serverId = slf.pickServer(clientId, msgName, req.GetPayload())
		if serverId == "" {
			slf.sendResponse(clientId, seqId, pb.Code_ERR, "game server is unavailable", nil)
			return
		}
		slf.gate.RememberServer(clientId, serverId)
	}
	slf.gate.TransferMessage2GameServer(clientId, serverId, msgName, seqId, req.GetPayload(), func(rsp *pb.TransferMessageRsp) error {
		if rsp == nil {
			rsp = &pb.TransferMessageRsp{
				BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR},
				Msg:     "empty transfer response",
				SeqId:   seqId,
			}
		}
		msgData := slf.processor.MakeMsg(uint16(pb.MsgType_MsgRes), rsp)
		return slf.wsService.SendMsg(clientId, msgData)
	})
}

func (slf *WsGateService) handleHeartbeat(clientId string, seqId int32, payload []byte) {
	req := &pb.C2SHeartbeat{}
	if err := proto.Unmarshal(payload, req); err != nil {
		slf.sendResponse(clientId, seqId, pb.Code_ERR, err.Error(), nil)
		return
	}
	body, err := proto.Marshal(&pb.S2CHeartbeat{No: req.GetNo()})
	if err != nil {
		slf.sendResponse(clientId, seqId, pb.Code_ERR, err.Error(), nil)
		return
	}
	slf.sendResponse(clientId, seqId, pb.Code_OK, "", body)
}

func (slf *WsGateService) sendResponse(clientId string, seqId int32, code pb.Code, msg string, payload []byte) {
	rsp := &pb.TransferMessageRsp{
		BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: code},
		Msg:     msg,
		SeqId:   seqId,
		Payload: payload,
	}
	msgData := slf.processor.MakeMsg(uint16(pb.MsgType_MsgRes), rsp)
	if err := slf.wsService.SendMsg(clientId, msgData); err != nil {
		log.SError("send transfer response error: ", err)
	}
}

func (slf *WsGateService) pushNotify(clientId, msgName string, payload []byte) error {
	notify := &pb.NotifyMessageInf{
		BaseReq: &pb.BaseReq{ClientId: clientId},
		MsgName: msgName,
		Payload: payload,
	}
	msgData := slf.processor.MakeMsg(uint16(pb.MsgType_MsgNotify), notify)
	return slf.wsService.SendMsg(clientId, msgData)
}

func (slf *WsGateService) OnConnected(clientId string) {
	log.SDebug("client id ", clientId, " is connected")
}

func (slf *WsGateService) OnDisconnected(clientId string) {
	log.SDebug("client id ", clientId, " is disconnected")
	_ = slf.gate.OnKick(clientId)
}

func (slf *WsGateService) OnUnknownMsg(clientId string, msg []byte) {
	log.SError("unknown websocket message client=", clientId, " size=", len(msg))
}

func (slf *WsGateService) loadGameServers() {
	group := config.GetConfigManager().GetWebSocketServiceGroup("game")
	if group == nil {
		return
	}
	for _, item := range group.Items {
		if item != nil && item.GetNo() != "" {
			slf.serverList = append(slf.serverList, item)
		}
	}
}

func (slf *WsGateService) pickServer(clientId, msgName string, payload []byte) string {
	if account := gate.ResolveAccount(msgName, payload); account != "" {
		if serverId := slf.gate.GetGameServerByAccount(account); serverId != "" {
			return serverId
		}
	}
	if serverId := slf.gate.ProvisionalServer(clientId); serverId != "" {
		return serverId
	}
	if len(slf.serverList) == 0 {
		return ""
	}
	return slf.serverList[rand.Intn(len(slf.serverList))].GetNo()
}
