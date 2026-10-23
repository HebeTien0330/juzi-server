package sync

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"pb"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Synchronizer 把游戏登记的根对象拍成点路径，并和上一份基线做差。
// 路径第一段是根名称，与客户端 RemoteSync.register 对应。
type Synchronizer struct {
	baseline map[string]string
	pending  map[string]string
}

func NewSynchronizer() *Synchronizer {
	return &Synchronizer{
		baseline: make(map[string]string),
		pending:  make(map[string]string),
	}
}

// Diff 对比 root 下的当前值。value 使用结构体、map 或切片。
func (s *Synchronizer) Diff(root string, value any) {
	if s == nil || root == "" {
		return
	}
	if s.baseline == nil {
		s.baseline = make(map[string]string)
	}
	if s.pending == nil {
		s.pending = make(map[string]string)
	}

	current := make(map[string]string)
	flatten(current, root, value)
	prefix := root + "."

	for path, encoded := range current {
		if s.baseline[path] != encoded {
			s.pending[path] = encoded
		}
	}
	for path := range s.baseline {
		if strings.HasPrefix(path, prefix) {
			if _, ok := current[path]; !ok {
				s.pending[path] = "null"
			}
			delete(s.baseline, path)
		}
	}
	for path, encoded := range current {
		s.baseline[path] = encoded
	}
}

func (s *Synchronizer) Take() map[string]string {
	if s == nil || len(s.pending) == 0 {
		return nil
	}
	out := s.pending
	s.pending = make(map[string]string)
	return out
}

// AttachRemote 把增量写进响应里名为 remote 的字段。没有该字段时跳过。
func AttachRemote(msg proto.Message, remote map[string]string) {
	if msg == nil || len(remote) == 0 {
		return
	}
	refl := msg.ProtoReflect()
	field := refl.Descriptor().Fields().ByName("remote")
	if field == nil || field.Kind() != protoreflect.MessageKind || field.IsMap() || field.IsList() {
		return
	}
	remoteMsg := refl.Mutable(field).Message()
	dataField := remoteMsg.Descriptor().Fields().ByName("data")
	if dataField == nil || !dataField.IsMap() {
		return
	}
	mapVal := remoteMsg.Mutable(dataField).Map()
	for k, v := range remote {
		mapVal.Set(protoreflect.ValueOfString(k).MapKey(), protoreflect.ValueOfString(v))
	}
}

func RemoteMessage(data map[string]string) *pb.Remote {
	if len(data) == 0 {
		return nil
	}
	return &pb.Remote{Data: data}
}

func flatten(out map[string]string, prefix string, value any) {
	if prefix == "" {
		return
	}
	rv := reflect.ValueOf(value)
	if !rv.IsValid() {
		return
	}
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}

	switch rv.Kind() {
	case reflect.Struct:
		if rv.Type() == reflect.TypeOf(time.Time{}) {
			out[prefix] = mustJSON(rv.Interface())
			return
		}
		rt := rv.Type()
		for i := 0; i < rv.NumField(); i++ {
			sf := rt.Field(i)
			if sf.PkgPath != "" || skipProtoField(sf.Name) {
				continue
			}
			child := rv.Field(i)
			if !child.CanInterface() {
				continue
			}
			flatten(out, prefix+"."+toLowerCamel(sf.Name), child.Interface())
		}
	case reflect.Map:
		for _, key := range rv.MapKeys() {
			flatten(out, prefix+"."+fmt.Sprint(key.Interface()), rv.MapIndex(key).Interface())
		}
	case reflect.Slice, reflect.Array:
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			out[prefix] = mustJSON(rv.Interface())
			return
		}
		for i := 0; i < rv.Len(); i++ {
			flatten(out, fmt.Sprintf("%s.%d", prefix, i), rv.Index(i).Interface())
		}
	default:
		out[prefix] = mustJSON(rv.Interface())
	}
}

func skipProtoField(name string) bool {
	switch name {
	case "state", "sizeCache", "unknownFields":
		return true
	default:
		return false
	}
}

func toLowerCamel(name string) string {
	r, size := utf8.DecodeRuneInString(name)
	if r == utf8.RuneError || size == 0 {
		return name
	}
	return string(unicode.ToLower(r)) + name[size:]
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "null"
	}
	return string(b)
}
