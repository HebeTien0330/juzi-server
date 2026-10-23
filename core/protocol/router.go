package protocol

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

type Handler func(clientID string, msg proto.Message) (proto.Message, error)

type Router struct {
	routes map[string]routeEntry
}

type routeEntry struct {
	msg     proto.Message
	handler Handler
}

func NewRouter() *Router {
	return &Router{routes: make(map[string]routeEntry)}
}

func (r *Router) Register(msgName string, msg proto.Message, handler Handler) {
	if r.routes == nil {
		r.routes = make(map[string]routeEntry)
	}
	r.routes[msgName] = routeEntry{msg: msg, handler: handler}
}

func (r *Router) Decode(payload []byte, msgName string) (proto.Message, error) {
	entry, ok := r.routes[msgName]
	if !ok {
		return nil, fmt.Errorf("unsupported message name: %s", msgName)
	}
	if entry.msg == nil {
		return nil, fmt.Errorf("no registered message type for %s", msgName)
	}

	msgType := reflect.TypeOf(entry.msg)
	if msgType.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("registered message must be a pointer for %s", msgName)
	}

	newMsg := reflect.New(msgType.Elem()).Interface().(proto.Message)
	if err := proto.Unmarshal(payload, newMsg); err != nil {
		return nil, err
	}
	return newMsg, nil
}

func (r *Router) Dispatch(clientid string, msgName string, msg proto.Message) (proto.Message, error) {
	entry, ok := r.routes[msgName]
	if !ok {
		return nil, fmt.Errorf("unsupported message name: %s", msgName)
	}
	if entry.handler == nil {
		return nil, fmt.Errorf("no handler registered for %s", msgName)
	}
	return entry.handler(clientid, msg)
}
