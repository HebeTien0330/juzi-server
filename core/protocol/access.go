package protocol

// 登录完成前允许转发到 Game 的消息名，由游戏在启动时登记。
var anonymous = map[string]struct{}{}

func AllowBeforeLogin(msgNames ...string) {
	for _, name := range msgNames {
		if name == "" {
			continue
		}
		anonymous[name] = struct{}{}
	}
}

func IsAnonymous(msgName string) bool {
	_, ok := anonymous[msgName]
	return ok
}
