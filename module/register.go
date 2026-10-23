package module

import "juzi-server/core/protocol"

// Registrar 由具体游戏在 init 里登记协议处理函数。
type Registrar func(router *protocol.Router)

var registrars []Registrar

func Register(fn Registrar) {
	if fn == nil {
		return
	}
	registrars = append(registrars, fn)
}

func Install(router *protocol.Router) {
	for _, fn := range registrars {
		fn(router)
	}
}
