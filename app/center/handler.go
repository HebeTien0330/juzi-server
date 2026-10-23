package center

import (
	"fmt"
	"reflect"
	"strings"

	"pb"

	"github.com/duanhf2012/origin/v2/log"
	"google.golang.org/protobuf/proto"
)

func (slf *Center) RPC_ReceiveRemoteMessage(req *pb.RpcMessageReq, rsp *pb.RpcMessageRsp) error {
	rsp.BaseRsp = &pb.InnerBaseRsp{Code: pb.Code_OK}
	if req == nil || req.GetMsgName() == "" {
		rsp.BaseRsp.Code = pb.Code_ERR
		rsp.Msg = "invalid remote message request"
		return nil
	}
	parts := strings.SplitN(req.GetMsgName(), ".", 2)
	if len(parts) != 2 {
		rsp.BaseRsp.Code = pb.Code_ERR
		rsp.Msg = "invalid msg name format, expected ServiceName.MethodName"
		return nil
	}
	service := slf.services[parts[0]]
	if service == nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		rsp.Msg = "service not found: " + parts[0]
		return nil
	}
	log.SInfo("invoke service method ", req.GetMsgName(), " from ", req.GetBaseReq().GetServerId())
	respMsg, err := invoke(service, parts[1], req.GetPayload())
	if err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		rsp.Msg = err.Error()
		return nil
	}
	if respMsg == nil {
		return nil
	}
	payload, err := proto.Marshal(respMsg)
	if err != nil {
		rsp.BaseRsp.Code = pb.Code_ERR
		rsp.Msg = err.Error()
		return nil
	}
	rsp.MsgName = string(respMsg.ProtoReflect().Descriptor().FullName())
	rsp.Payload = payload
	return nil
}

func invoke(service any, methodName string, payload []byte) (proto.Message, error) {
	method := reflect.ValueOf(service).MethodByName(methodName)
	if !method.IsValid() {
		return nil, fmt.Errorf("method not found: %s", methodName)
	}
	methodType := method.Type()
	if methodType.NumIn() != 1 || methodType.In(0).Kind() != reflect.Ptr {
		return nil, fmt.Errorf("method %s must take one proto pointer", methodName)
	}
	arg := reflect.New(methodType.In(0).Elem())
	reqMsg, ok := arg.Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("method %s request is not a proto message", methodName)
	}
	if len(payload) > 0 {
		if err := proto.Unmarshal(payload, reqMsg); err != nil {
			return nil, err
		}
	}
	results := method.Call([]reflect.Value{arg})
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}
	if len(results) == 0 || results[0].IsNil() {
		return nil, nil
	}
	resp, ok := results[0].Interface().(proto.Message)
	if !ok {
		return nil, fmt.Errorf("method %s return value is not a proto message", methodName)
	}
	return resp, nil
}
