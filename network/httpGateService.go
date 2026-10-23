package network

import (
	"fmt"
	"net/http"

	"juzi-server/config"

	"github.com/duanhf2012/origin/v2/log"
	"github.com/duanhf2012/origin/v2/node"
	"github.com/duanhf2012/origin/v2/service"
	"github.com/duanhf2012/origin/v2/sysservice/httpservice"
)

func init() {
	log.Info("setup http gate service")
	node.Setup(&httpservice.HttpService{})
	node.Setup(&HttpGateService{})
}

type HttpGateService struct {
	service.Service
	configMgr *config.ConfigManager
}

type serverInfo struct {
	ServerName string `json:"serverName"`
	ServerAddr string `json:"serverAddr"`
	ServerPort string `json:"serverPort"`
	Version    string `json:"version"`
}

type httpResponse struct {
	ServerInfo serverInfo `json:"serverInfo"`
}

func (slf *HttpGateService) OnInit() error {
	httpService := node.GetService("HttpService").(*httpservice.HttpService)
	router := httpservice.NewHttpHttpRouter()
	httpService.SetHttpRouter(router, slf.GetEventHandler())
	httpService.SetAllowCORS(httpservice.NewAllowCORSHeader())
	router.GET("/", slf.handleVersion)
	router.GET("/version/", slf.handleVersion)
	slf.configMgr = config.GetConfigManager()
	log.Info("http gate service start ok")
	return nil
}

func (slf *HttpGateService) handleVersion(session *httpservice.HttpSession) {
	session.WriteJsonDone(http.StatusOK, &httpResponse{ServerInfo: slf.queryServer()})
}

func (slf *HttpGateService) queryServer() serverInfo {
	info := serverInfo{Version: slf.configMgr.GetVersion()}
	item := slf.configMgr.GetHttpServiceItem("wsGate", "wsGate-1")
	if item == nil {
		log.SError("http service item wsGate/wsGate-1 not found")
		return info
	}
	info.ServerName = item.GetId()
	info.ServerAddr = item.GetHost()
	info.ServerPort = fmt.Sprintf("%d", item.GetPort())
	return info
}
