package gate

import (
	"fmt"

	"pb"

	"github.com/duanhf2012/origin/v2/log"
)

func (slf *Gate) RPC_SwitchGameServer(req *pb.SwicthGameServerReq, rsp *pb.SwicthGameServerRsp) error {
	if rsp == nil {
		return fmt.Errorf("switch game server response is nil")
	}
	rsp.BaseRsp = &pb.BaseRsp{Code: pb.Code_OK}
	if req == nil || req.GetBaseReq() == nil {
		return fmt.Errorf("invalid switch game server request")
	}
	clientId := req.GetBaseReq().GetClientId()
	userId := slf.GetUserId(clientId)
	if clientId == "" || userId == "" {
		return fmt.Errorf("switch game server player not found")
	}
	rsp.BaseRsp.ClientId = clientId

	originServerId := req.GetOriginServerId()
	targetServerId := req.GetTargetServerId()
	if originServerId == "" || targetServerId == "" {
		return fmt.Errorf("switch game server id is empty")
	}
	if slf.GetCenterServerId(originServerId) != slf.GetCenterServerId(targetServerId) {
		return fmt.Errorf("switch game server center mismatch")
	}

	exportRsp := &pb.ExportPlayerRsp{}
	exportReq := &pb.ExportPlayerReq{
		BaseReq:  &pb.InnerBaseReq{ServerId: slf.GetServerId()},
		UserId:   userId,
		ClientId: clientId,
	}
	if err := slf.CallNode(originServerId, "Game.RPC_ExportPlayer", exportReq, exportRsp); err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		return err
	}
	if exportRsp.GetBaseRsp().GetCode() != pb.Code_OK || exportRsp.GetTransferReq() == nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		return fmt.Errorf("export player failed")
	}

	transferReq := exportRsp.GetTransferReq()
	transferReq.ClientId = clientId
	transferRsp := &pb.TransferPlayerRsp{}
	if err := slf.CallNode(targetServerId, "Game.RPC_TransferPlayer", transferReq, transferRsp); err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		return err
	}
	if transferRsp.GetBaseRsp().GetCode() != pb.Code_OK {
		rsp.BaseRsp.Code = pb.Code_ERR
		return fmt.Errorf("transfer player failed")
	}

	if msgName := req.GetMsgName(); msgName != "" {
		actionRsp, err := slf.transferMessageSync(clientId, targetServerId, msgName, req.GetPayload())
		if err != nil {
			rsp.BaseRsp.Code = pb.Code_ERR
			return err
		}
		if actionRsp.GetBaseRsp().GetCode() != pb.Code_OK {
			rsp.BaseRsp.Code = pb.Code_ERR
			return fmt.Errorf("invoke switch action failed: %s", actionRsp.GetMsg())
		}
		rsp.Payload = actionRsp.GetPayload()
	}

	info := slf.GetClientInfo(userId)
	if info == nil || info.GetClientId() != clientId || info.GetGameServerId() != originServerId {
		rsp.BaseRsp.Code = pb.Code_ERR
		return fmt.Errorf("switch game server update client failed")
	}
	info.GameServerId = targetServerId
	log.SInfo("player ", userId, " switched game server from ", originServerId, " to ", targetServerId)
	return nil
}
