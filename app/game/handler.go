package game

import (
	"fmt"

	datasync "juzi-server/core/sync"
	"pb"

	"github.com/duanhf2012/origin/v2/log"
	networkprocessor "github.com/duanhf2012/origin/v2/network/processor"
	"github.com/duanhf2012/origin/v2/rpc"
	"google.golang.org/protobuf/proto"
)

func (slf *Game) RPC_ReceiveTransferMessage(responder rpc.RequestHandler, req *pb.TransferMessageReq) error {
	if req == nil || req.BaseReq == nil {
		responder(&pb.TransferMessageRsp{
			BaseRsp: &pb.BaseRsp{Code: pb.Code_ERR},
			Msg:     "invalid transfer request",
		}, "")
		return fmt.Errorf("invalid transfer request")
	}
	result := &transferResult{rsp: &pb.TransferMessageRsp{SeqId: req.GetSeqId()}}
	slf.AsyncDo(func() bool {
		result.err = slf.handleTransferMessage(req, result.rsp)
		return true
	}, func(asyncErr error) {
		if asyncErr != nil && result.err == nil {
			result.err = asyncErr
		}
		var rpcErr rpc.RpcError
		if result.err != nil {
			rpcErr = rpc.RpcError(result.err.Error())
		}
		responder(result.rsp, rpcErr)
	})
	return nil
}

type transferResult struct {
	rsp *pb.TransferMessageRsp
	err error
}

func (slf *Game) RPC_TransferMessageSync(req *pb.TransferMessageReq, rsp *pb.TransferMessageRsp) error {
	if rsp == nil {
		return fmt.Errorf("transfer message response is nil")
	}
	return slf.handleTransferMessage(req, rsp)
}

func (slf *Game) handleTransferMessage(req *pb.TransferMessageReq, rsp *pb.TransferMessageRsp) error {
	if req == nil || req.GetBaseReq() == nil {
		rsp.BaseRsp = &pb.BaseRsp{Code: pb.Code_ERR}
		rsp.Msg = "invalid transfer request"
		return fmt.Errorf("invalid transfer request")
	}
	rsp.SeqId = req.GetSeqId()
	clientId := req.GetBaseReq().GetClientId()
	log.SInfo("receive transfer message from client ", clientId, ", msg name: ", req.GetMsgName())

	protoMsg, err := slf.router.Decode(req.GetPayload(), req.GetMsgName())
	if err != nil {
		decoded, decodeErr := slf.processor.Unmarshal(clientId, req.GetPayload())
		if decodeErr != nil {
			rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR}
			rsp.Msg = err.Error()
			return err
		}
		pack, ok := decoded.(*networkprocessor.PBPackInfo)
		if !ok || pack.GetMsg() == nil {
			rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR}
			rsp.Msg = err.Error()
			return err
		}
		protoMsg = pack.GetMsg()
	}

	respMsg, err := slf.router.Dispatch(clientId, req.GetMsgName(), protoMsg)
	if err != nil {
		rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR}
		rsp.Msg = err.Error()
		return err
	}
	if respMsg != nil {
		if manager := slf.ActorManager(); manager != nil {
			if a := manager.GetByClient(clientId); a != nil {
				datasync.AttachRemote(respMsg, a.FlushRemote())
			}
		}
		payload, err := proto.Marshal(respMsg)
		if err != nil {
			rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_ERR}
			rsp.Msg = err.Error()
			return err
		}
		rsp.Payload = payload
	}
	rsp.BaseRsp = &pb.BaseRsp{ClientId: clientId, Code: pb.Code_OK}
	return nil
}
