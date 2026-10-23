package gate

import (
	"fmt"

	"juzi-server/config"
	"pb"

	"github.com/duanhf2012/origin/v2/log"
)

func (slf *Gate) TransferMessage2GameServer(clientId, serverId, msgName string, seqId int32, payload []byte, onResponse func(*pb.TransferMessageRsp) error) {
	if serverId == "" {
		serverId = slf.GetGameServerId(slf.GetUserId(clientId))
	}
	if serverId == "" {
		serverId = slf.ProvisionalServer(clientId)
	}
	if serverId == "" {
		if onResponse != nil {
			_ = onResponse(&pb.TransferMessageRsp{
				BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR},
				Msg:     "game server is unavailable",
				SeqId:   seqId,
			})
		}
		return
	}

	req := &pb.TransferMessageReq{
		BaseReq: &pb.BaseReq{ClientId: clientId},
		MsgName: msgName,
		SeqId:   seqId,
		Payload: payload,
	}
	err := slf.AsyncCallNode(serverId, "Game.RPC_ReceiveTransferMessage", req, func(rsp *pb.TransferMessageRsp, err error) {
		if err != nil {
			if onResponse != nil {
				_ = onResponse(&pb.TransferMessageRsp{
					BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR},
					Msg:     err.Error(),
					SeqId:   seqId,
				})
			}
			return
		}
		if rsp == nil {
			rsp = &pb.TransferMessageRsp{
				BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR},
				Msg:     "empty response",
				SeqId:   seqId,
			}
		}
		if rsp.BaseRsp == nil {
			rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR}
		}
		if rsp.SeqId == 0 {
			rsp.SeqId = seqId
		}
		if msgName == "C2SLogout" && rsp.GetBaseRsp().GetCode() == pb.Code_OK {
			slf.ClearSessionByClient(clientId)
		}
		if onResponse != nil {
			if err := onResponse(rsp); err != nil {
				log.SError("send websocket response error ", err)
			}
		}
	})
	if err != nil && onResponse != nil {
		_ = onResponse(&pb.TransferMessageRsp{
			BaseRsp: &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR},
			Msg:     err.Error(),
			SeqId:   seqId,
		})
	}
}

func (slf *Gate) RPC_ReceiveNotifyMessage(notifyInf *pb.NotifyMessageInf) error {
	if notifyInf == nil || notifyInf.BaseReq == nil {
		return fmt.Errorf("invalid notify message")
	}
	clientId := notifyInf.BaseReq.ClientId
	if _, exists := slf.players[clientId]; !exists {
		return nil
	}
	if notifyInf.MsgName == "" {
		return fmt.Errorf("msgName is empty")
	}
	if slf.notifyPusher == nil {
		return fmt.Errorf("notify pusher not initialized")
	}
	return slf.notifyPusher(clientId, notifyInf.MsgName, notifyInf.Payload)
}

func (slf *Gate) transferMessageSync(clientId, serverId, msgName string, payload []byte) (*pb.TransferMessageRsp, error) {
	req := &pb.TransferMessageReq{
		BaseReq: &pb.BaseReq{ClientId: clientId},
		MsgName: msgName,
		Payload: payload,
	}
	rsp := &pb.TransferMessageRsp{}
	if err := slf.CallNode(serverId, "Game.RPC_TransferMessageSync", req, rsp); err != nil {
		return nil, err
	}
	return rsp, nil
}

func (slf *Gate) GetCenterServerId(gameServerId string) string {
	configMgr := config.GetConfigManager()
	gameItems := configMgr.GetWebSocketServiceGroup("game")
	if gameItems == nil {
		return ""
	}
	var master string
	for _, item := range gameItems.Items {
		if item != nil && item.No == gameServerId {
			master = item.Master
			break
		}
	}
	if master == "" {
		return ""
	}
	if centerItem := configMgr.GetWebSocketServiceItem("center", master); centerItem != nil && centerItem.No != "" {
		return centerItem.No
	}
	return master
}
