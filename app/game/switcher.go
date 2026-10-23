package game

import (
	"errors"
	"fmt"

	"juzi-server/core/actor"
	"pb"

	"google.golang.org/protobuf/proto"
)

type SwitchAction struct {
	MsgName string
	Req     proto.Message
	Rsp     proto.Message
}

func (slf *Game) SwitchServer(a *actor.Actor, targetServerId string, action *SwitchAction) error {
	if a == nil {
		return errors.New("actor is nil")
	}
	if targetServerId == "" || targetServerId == slf.GetServerId() {
		return errors.New("invalid target server")
	}
	switchReq := &pb.SwicthGameServerReq{
		BaseReq:        &pb.BaseReq{ClientId: a.ClientId},
		OriginServerId: slf.GetServerId(),
		TargetServerId: targetServerId,
	}
	if action != nil {
		switchReq.MsgName = action.MsgName
		if action.Req != nil {
			payload, err := proto.Marshal(action.Req)
			if err != nil {
				return err
			}
			switchReq.Payload = payload
		}
	}
	switchRsp := &pb.SwicthGameServerRsp{}
	if err := slf.CallNode(slf.gateNodeId(), "Gate.RPC_SwitchGameServer", switchReq, switchRsp); err != nil {
		return err
	}
	if switchRsp.GetBaseRsp().GetCode() != pb.Code_OK {
		return errors.New("switch game server failed")
	}
	if action != nil && action.Rsp != nil && len(switchRsp.GetPayload()) > 0 {
		if err := proto.Unmarshal(switchRsp.GetPayload(), action.Rsp); err != nil {
			return fmt.Errorf("unmarshal switch action rsp failed: %w", err)
		}
	}
	return nil
}

func (slf *Game) RPC_ExportPlayer(req *pb.ExportPlayerReq, rsp *pb.ExportPlayerRsp) error {
	rsp.BaseRsp = &pb.InnerBaseRsp{Code: pb.Code_OK}
	manager := slf.ActorManager()
	if manager == nil || req == nil || req.GetUserId() == "" {
		rsp.BaseRsp.Code = pb.Code_ERR
		return nil
	}
	a, snapshot, err := manager.Export(req.GetUserId())
	if err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		return nil
	}
	rsp.TransferReq = &pb.TransferPlayerReq{
		BaseReq:      &pb.InnerBaseReq{ServerId: slf.GetServerId()},
		UserId:       a.UserId,
		Account:      a.Account,
		ClientId:     req.GetClientId(),
		SessionToken: a.Token,
		Snapshot:     snapshot,
	}
	return nil
}

func (slf *Game) RPC_TransferPlayer(req *pb.TransferPlayerReq, rsp *pb.TransferPlayerRsp) error {
	rsp.BaseRsp = &pb.InnerBaseRsp{Code: pb.Code_OK}
	manager := slf.ActorManager()
	if manager == nil || req == nil || req.GetUserId() == "" {
		rsp.BaseRsp.Code = pb.Code_ERR
		return nil
	}
	if _, err := manager.Import(req.GetUserId(), req.GetAccount(), req.GetClientId(), req.GetSessionToken(), req.GetSnapshot()); err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		return nil
	}
	return nil
}
