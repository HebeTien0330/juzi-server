package gate

import (
	"errors"

	"pb"

	"github.com/duanhf2012/origin/v2/log"
	"google.golang.org/protobuf/proto"
)

var accountResolver func(msgName string, payload []byte) string

// SetAccountResolver 让游戏从登录包里取出账号，便于把同一账号送回原来的 Game。
func SetAccountResolver(fn func(msgName string, payload []byte) string) {
	accountResolver = fn
}

func ResolveAccount(msgName string, payload []byte) string {
	if accountResolver == nil {
		return ""
	}
	return accountResolver(msgName, payload)
}

func (slf *Gate) GetGameServerByAccount(account string) string {
	if account == "" {
		return ""
	}
	userId := slf.accounts[account]
	if userId == "" {
		return ""
	}
	info := slf.clients[userId]
	if info == nil {
		return ""
	}
	return info.GetGameServerId()
}

func (slf *Gate) bindSession(account, userId string, clientInfo *pb.ClientInfo) {
	if userId == "" || clientInfo == nil || clientInfo.GetClientId() == "" {
		return
	}
	slf.cancelGrace(userId)
	if old := slf.clients[userId]; old != nil && old.ClientId != "" && old.ClientId != clientInfo.ClientId {
		delete(slf.players, old.ClientId)
	}
	slf.players[clientInfo.ClientId] = userId
	slf.clients[userId] = clientInfo
	if account != "" {
		slf.accounts[account] = userId
		slf.userAccount[userId] = account
	}
}

func (slf *Gate) bindReconnect(userId, clientId string) error {
	info := slf.clients[userId]
	if info == nil || info.GetGameServerId() == "" {
		return errors.New("session expired")
	}
	slf.cancelGrace(userId)
	if info.ClientId != "" && info.ClientId != clientId {
		delete(slf.players, info.ClientId)
	}
	info.ClientId = clientId
	slf.players[clientId] = userId
	delete(slf.provisional, clientId)
	return nil
}

func (slf *Gate) ClearSessionByClient(clientId string) {
	userId := slf.GetUserId(clientId)
	if userId == "" {
		return
	}
	slf.clearSessionByUserId(userId)
}

func (slf *Gate) clearSessionByUserId(userId string) {
	slf.cancelGrace(userId)
	info := slf.clients[userId]
	if info != nil && info.ClientId != "" {
		delete(slf.players, info.ClientId)
		delete(slf.provisional, info.ClientId)
	}
	delete(slf.clients, userId)
	if account := slf.userAccount[userId]; account != "" {
		if slf.accounts[account] == userId {
			delete(slf.accounts, account)
		}
		delete(slf.userAccount, userId)
	}
}

func (slf *Gate) startGrace(userId string) {
	if userId == "" {
		return
	}
	slf.cancelGrace(userId)
	timerId := new(uint64)
	slf.graceTimers[userId] = timerId
	slf.SafeAfterFunc(timerId, ReconnectGrace, userId, func(_ uint64, data interface{}) {
		uid, _ := data.(string)
		delete(slf.graceTimers, uid)
		slf.onGraceExpired(uid)
	})
}

func (slf *Gate) cancelGrace(userId string) {
	timerId, ok := slf.graceTimers[userId]
	if !ok {
		return
	}
	delete(slf.graceTimers, userId)
	if timerId != nil && *timerId != 0 {
		slf.CancelTimerId(timerId)
	}
}

func (slf *Gate) onGraceExpired(userId string) {
	info := slf.clients[userId]
	if info == nil || info.GetClientId() != "" {
		return
	}
	gameServerId := info.GetGameServerId()
	slf.clearSessionByUserId(userId)
	if gameServerId == "" {
		return
	}
	log.SInfo("gate reconnect grace expired, logout user=", userId)
	payload, err := proto.Marshal(&pb.C2SLogout{UserId: userId})
	if err != nil {
		log.SError("marshal grace logout req error ", err)
		return
	}
	req := &pb.TransferMessageReq{
		BaseReq: &pb.BaseReq{},
		MsgName: "C2SLogout",
		Payload: payload,
	}
	if err := slf.AsyncCallNode(gameServerId, "Game.RPC_ReceiveTransferMessage", req, func(_ *pb.TransferMessageRsp, err error) {
		if err != nil {
			log.SError("grace logout callback error user=", userId, " err=", err)
		}
	}); err != nil {
		log.SError("grace logout call error user=", userId, " err=", err)
	}
}

func (slf *Gate) OnReconnect(clientId, userId string, req *pb.TransferMessageReq, onComplete func(*pb.S2CReconnect, error)) error {
	if userId == "" {
		return errors.New("userId is empty")
	}
	info := slf.GetClientInfo(userId)
	if info == nil || info.GetGameServerId() == "" {
		return errors.New("session expired")
	}
	if req.BaseReq == nil {
		req.BaseReq = &pb.BaseReq{}
	}
	req.BaseReq.ClientId = clientId
	gameServerId := info.GetGameServerId()
	log.SInfo("player ", userId, " reconnect, clientid ", clientId, " gameServer ", gameServerId)
	return slf.AsyncCallNode(gameServerId, "Game.RPC_ReceiveTransferMessage", req, func(rsp *pb.TransferMessageRsp, err error) {
		if err != nil {
			if onComplete != nil {
				onComplete(nil, err)
			}
			return
		}
		reconnectRsp := &pb.S2CReconnect{}
		if unmarshalErr := proto.Unmarshal(rsp.GetPayload(), reconnectRsp); unmarshalErr != nil {
			if onComplete != nil {
				onComplete(nil, unmarshalErr)
			}
			return
		}
		if rsp.GetBaseRsp() != nil && rsp.GetBaseRsp().GetCode() != pb.Code_OK {
			if onComplete != nil {
				onComplete(nil, errors.New(rsp.GetMsg()))
			}
			return
		}
		if reconnectRsp.GetCode() != pb.Code_OK {
			if onComplete != nil {
				onComplete(reconnectRsp, nil)
			}
			return
		}
		if bindErr := slf.bindReconnect(userId, clientId); bindErr != nil {
			if onComplete != nil {
				onComplete(nil, bindErr)
			}
			return
		}
		if onComplete != nil {
			onComplete(reconnectRsp, nil)
		}
	})
}

func (slf *Gate) OnKick(clientId string) error {
	userId := slf.GetUserId(clientId)
	if userId == "" {
		slf.ForgetClient(clientId)
		return nil
	}
	info := slf.GetClientInfo(userId)
	if info == nil {
		delete(slf.players, clientId)
		return nil
	}
	if info.ClientId != "" && info.ClientId != clientId {
		delete(slf.players, clientId)
		return nil
	}
	gameServerId := info.GetGameServerId()
	delete(slf.players, clientId)
	info.ClientId = ""
	slf.startGrace(userId)
	if gameServerId == "" {
		return errors.New("game server id is empty")
	}
	payload, err := proto.Marshal(&pb.C2SDisconnect{UserId: userId})
	if err != nil {
		return err
	}
	req := &pb.TransferMessageReq{
		BaseReq: &pb.BaseReq{ClientId: clientId},
		MsgName: "C2SDisconnect",
		Payload: payload,
	}
	return slf.AsyncCallNode(gameServerId, "Game.RPC_ReceiveTransferMessage", req, func(rsp *pb.TransferMessageRsp, err error) {
		if err != nil {
			log.SError("game server disconnect callback error ", err)
		}
	})
}
