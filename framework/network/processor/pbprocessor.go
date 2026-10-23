package processor

import (
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/duanhf2012/origin/v2/util/bytespool"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

type MessageInfo struct {
	msgType    reflect.Type
	msgHandler MessageHandler
}

type MessageHandler func(clientId string, msg proto.Message)
type ConnectHandler func(clientId string)
type UnknownMessageHandler func(clientId string, msg []byte)

const MsgTypeSize = 2

type PBProcessor struct {
	mapMsg       map[uint16][]MessageInfo
	LittleEndian bool

	unknownMessageHandler UnknownMessageHandler
	connectHandler        ConnectHandler
	disconnectHandler     ConnectHandler
	bytespool.IBytesMemPool
}

type PBPackInfo struct {
	typ     uint16
	msg     proto.Message
	rawMsg  []byte
	handler MessageHandler
}

func NewPBProcessor() *PBProcessor {
	processor := &PBProcessor{mapMsg: map[uint16][]MessageInfo{}}
	processor.IBytesMemPool = bytespool.NewMemAreaPool()
	return processor
}

func (pbProcessor *PBProcessor) SetByteOrder(littleEndian bool) {
	pbProcessor.LittleEndian = littleEndian
}

func (slf *PBPackInfo) GetPackType() uint16 {
	return slf.typ
}

func (slf *PBPackInfo) GetMsg() proto.Message {
	return slf.msg
}

// MsgRoute must goroutine safe
func (pbProcessor *PBProcessor) MsgRoute(clientId string, msg interface{}, recyclerReaderBytes func(data []byte)) error {
	pPackInfo := msg.(*PBPackInfo)
	defer recyclerReaderBytes(pPackInfo.rawMsg)

	if pPackInfo.handler == nil {
		return fmt.Errorf("cannot find msgtype %d is register", pPackInfo.typ)
	}

	pPackInfo.handler(clientId, pPackInfo.msg)
	return nil
}

// Unmarshal must goroutine safe
func (pbProcessor *PBProcessor) Unmarshal(clientId string, data []byte) (interface{}, error) {
	return pbProcessor.UnmarshalWithOutRelease(clientId, data)
}

// UnmarshalWithOutRelease not release data
func (pbProcessor *PBProcessor) UnmarshalWithOutRelease(clientId string, data []byte) (interface{}, error) {
	var msgType uint16
	if pbProcessor.LittleEndian == true {
		msgType = binary.LittleEndian.Uint16(data[:2])
	} else {
		msgType = binary.BigEndian.Uint16(data[:2])
	}

	infos, ok := pbProcessor.mapMsg[msgType]
	if ok == false || len(infos) == 0 {
		return nil, fmt.Errorf("cannot find register %d msgtype", msgType)
	}

	info, err := pbProcessor.matchMessageInfo(infos, data[2:])
	if err != nil {
		return nil, err
	}

	msg := reflect.New(info.msgType.Elem()).Interface()
	protoMsg := msg.(proto.Message)
	err = proto.Unmarshal(data[2:], protoMsg)
	if err != nil {
		return nil, err
	}

	return &PBPackInfo{typ: msgType, msg: protoMsg, rawMsg: data, handler: info.msgHandler}, nil
}

// Marshal must goroutine safe
func (pbProcessor *PBProcessor) Marshal(clientId string, msg interface{}) ([]byte, error) {
	pMsg := msg.(*PBPackInfo)

	var err error
	if pMsg.msg != nil {
		pMsg.rawMsg, err = proto.Marshal(pMsg.msg)
		if err != nil {
			return nil, err
		}
	}

	buff := make([]byte, 2, len(pMsg.rawMsg)+MsgTypeSize)
	if pbProcessor.LittleEndian == true {
		binary.LittleEndian.PutUint16(buff[:2], pMsg.typ)
	} else {
		binary.BigEndian.PutUint16(buff[:2], pMsg.typ)
	}

	buff = append(buff, pMsg.rawMsg...)
	return buff, nil
}

func (pbProcessor *PBProcessor) Register(msgType uint16, msg proto.Message, handle MessageHandler) {
	var info MessageInfo

	info.msgType = reflect.TypeOf(msg.(proto.Message))
	info.msgHandler = handle
	pbProcessor.mapMsg[msgType] = append(pbProcessor.mapMsg[msgType], info)
}

func (pbProcessor *PBProcessor) matchMessageInfo(infos []MessageInfo, payload []byte) (MessageInfo, error) {
	if len(infos) == 1 {
		return infos[0], nil
	}

	fieldNum, wireType, n := protowire.ConsumeTag(payload)
	if n < 0 {
		return MessageInfo{}, fmt.Errorf("invalid protobuf payload")
	}

	for _, info := range infos {
		msg := reflect.New(info.msgType.Elem()).Interface().(proto.Message)
		field := msg.ProtoReflect().Descriptor().Fields().ByNumber(fieldNum)
		if field == nil {
			continue
		}

		if expectedWireType(field.Kind()) == wireType {
			return info, nil
		}
	}

	return MessageInfo{}, fmt.Errorf("cannot match protobuf payload for msgtype")
}

func expectedWireType(kind protoreflect.Kind) protowire.Type {
	switch kind {
	case protoreflect.BoolKind,
		protoreflect.EnumKind,
		protoreflect.Int32Kind,
		protoreflect.Int64Kind,
		protoreflect.Uint32Kind,
		protoreflect.Uint64Kind,
		protoreflect.Sint32Kind,
		protoreflect.Sint64Kind:
		return protowire.VarintType
	case protoreflect.Fixed32Kind,
		protoreflect.Sfixed32Kind,
		protoreflect.FloatKind:
		return protowire.Fixed32Type
	case protoreflect.Fixed64Kind,
		protoreflect.Sfixed64Kind,
		protoreflect.DoubleKind:
		return protowire.Fixed64Type
	case protoreflect.StringKind,
		protoreflect.BytesKind,
		protoreflect.MessageKind:
		return protowire.BytesType
	default:
		return protowire.BytesType
	}
}

func (pbProcessor *PBProcessor) MakeMsg(msgType uint16, protoMsg proto.Message) *PBPackInfo {
	return &PBPackInfo{typ: msgType, msg: protoMsg}
}

func (pbProcessor *PBProcessor) MakeRawMsg(msgType uint16, msg []byte) *PBPackInfo {
	return &PBPackInfo{typ: msgType, rawMsg: msg}
}

func (pbProcessor *PBProcessor) UnknownMsgRoute(clientId string, msg interface{}, recyclerReaderBytes func(data []byte)) {
	data, ok := msg.([]byte)
	if ok == false {
		return
	}

	if pbProcessor.unknownMessageHandler != nil {
		pbProcessor.unknownMessageHandler(clientId, data)
	}

	recyclerReaderBytes(data)
}

func (pbProcessor *PBProcessor) ConnectedRoute(clientId string) {
	pbProcessor.connectHandler(clientId)
}

func (pbProcessor *PBProcessor) DisConnectedRoute(clientId string) {
	pbProcessor.disconnectHandler(clientId)
}

func (pbProcessor *PBProcessor) RegisterUnknownMsg(unknownMessageHandler UnknownMessageHandler) {
	pbProcessor.unknownMessageHandler = unknownMessageHandler
}

func (pbProcessor *PBProcessor) RegisterConnected(connectHandler ConnectHandler) {
	pbProcessor.connectHandler = connectHandler
}

func (pbProcessor *PBProcessor) RegisterDisConnected(disconnectHandler ConnectHandler) {
	pbProcessor.disconnectHandler = disconnectHandler
}
