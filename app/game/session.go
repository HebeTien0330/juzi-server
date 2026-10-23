package game

import (
	"juzi-server/core/actor"
	datasync "juzi-server/core/sync"
	"pb"

	"github.com/duanhf2012/origin/v2/log"
	"google.golang.org/protobuf/proto"
)

func (slf *Game) registerSessionRoutes() {
	slf.router.Register("C2SLogout", &pb.C2SLogout{}, slf.onLogout)
	slf.router.Register("C2SDisconnect", &pb.C2SDisconnect{}, slf.onDisconnect)
	slf.router.Register("C2SReconnect", &pb.C2SReconnect{}, slf.onReconnect)
}

func (slf *Game) onLogout(clientId string, msg proto.Message) (proto.Message, error) {
	req := msg.(*pb.C2SLogout)
	manager := slf.ActorManager()
	if manager == nil {
		return &pb.S2CLogout{Code: pb.Code_ERR}, nil
	}
	userId := req.GetUserId()
	if userId == "" {
		if a := manager.GetByClient(clientId); a != nil {
			userId = a.UserId
		}
	}
	slf.CancelLogout(userId)
	if a := manager.Leave(userId); a != nil {
		_ = a.SaveModules()
	}
	return &pb.S2CLogout{Code: pb.Code_OK}, nil
}

func (slf *Game) onDisconnect(clientId string, msg proto.Message) (proto.Message, error) {
	req := msg.(*pb.C2SDisconnect)
	manager := slf.ActorManager()
	if manager == nil {
		return &pb.S2CDisconnect{Code: pb.Code_ERR}, nil
	}
	userId := req.GetUserId()
	a := manager.MarkDisconnected(userId)
	if a == nil {
		return &pb.S2CDisconnect{Code: pb.Code_OK}, nil
	}
	slf.ScheduleLogout(userId)
	log.SInfo("player disconnected, keep session user=", userId, " client=", clientId)
	return &pb.S2CDisconnect{Code: pb.Code_OK}, nil
}

func (slf *Game) onReconnect(clientId string, msg proto.Message) (proto.Message, error) {
	req := msg.(*pb.C2SReconnect)
	manager := slf.ActorManager()
	if manager == nil {
		return &pb.S2CReconnect{Code: pb.Code_ERR}, nil
	}
	a, token, err := manager.Reconnect(req.GetUserId(), clientId, req.GetSessionToken())
	if err != nil {
		return &pb.S2CReconnect{Code: pb.Code_ERR}, nil
	}
	slf.CancelLogout(a.UserId)
	return &pb.S2CReconnect{
		Code:         pb.Code_OK,
		Clientid:     clientId,
		UserId:       a.UserId,
		SessionToken: token,
		Remote:       datasync.RemoteMessage(a.FlushRemote()),
	}, nil
}

func (slf *Game) ScheduleLogout(userId string) {
	if userId == "" {
		return
	}
	slf.CancelLogout(userId)
	timerId := new(uint64)
	slf.graceTimers[userId] = timerId
	slf.SafeAfterFunc(timerId, actor.ReconnectGrace, userId, func(_ uint64, data interface{}) {
		uid, _ := data.(string)
		delete(slf.graceTimers, uid)
		slf.onGraceLogout(uid)
	})
}

func (slf *Game) CancelLogout(userId string) {
	timerId, ok := slf.graceTimers[userId]
	if !ok {
		return
	}
	delete(slf.graceTimers, userId)
	if timerId != nil && *timerId != 0 {
		slf.CancelTimerId(timerId)
	}
}

func (slf *Game) onGraceLogout(userId string) {
	manager := slf.ActorManager()
	if manager == nil {
		return
	}
	a := manager.Get(userId)
	if a == nil || !a.Disconnected {
		return
	}
	log.SInfo("reconnect grace expired, logout user=", userId)
	if left := manager.Leave(userId); left != nil {
		_ = left.SaveModules()
	}
}
