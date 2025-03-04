package relay

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/ftp"
	"github.com/jm33-m0/emp3r0r/core/internal/cc/base/network"
	"github.com/jm33-m0/emp3r0r/core/internal/def"
	"github.com/jm33-m0/emp3r0r/core/internal/live"
	"github.com/jm33-m0/emp3r0r/core/internal/transport"
	"github.com/jm33-m0/emp3r0r/core/lib/logging"
	"github.com/jm33-m0/emp3r0r/core/lib/netutil"
	"github.com/jm33-m0/emp3r0r/core/lib/util"
)

// This server handles relayed HTTP requests from C2, it listens on WireGuard interface
func RelayHTTP2Server() {
	time.Sleep(3 * time.Second)
	r := mux.NewRouter()
	r.HandleFunc(fmt.Sprintf("/%s/{api}/{token}", transport.WebRoot), dispatcher)
	listenAddr := fmt.Sprintf("%s:%d", netutil.WgOperatorIP, netutil.WgRelayedHTTPPort)
	err := http.ListenAndServeTLS(listenAddr, transport.OperatorServerCrtFile, transport.OperatorServerKeyFile, r)
	if err != nil {
		logging.Errorf("Failed to start HTTP server: %v", err)
	}
}

func dispatcher(wrt http.ResponseWriter, req *http.Request) {
	logging.Debugf("Relayed request: %s %s", req.Method, req.URL.Path)
	logging.Debugf("Relayed request headers: %v", req.Header)
	api := mux.Vars(req)["api"]
	token := mux.Vars(req)["token"]
	logging.Debugf("Got relayed request from C2: API: %s, token: %s", api, token)

	// Setup H2Conn for port mapping.
	proxyConn := new(def.H2Conn)
	network.ProxyStream.H2x = proxyConn

	// match API names
	api = transport.WebRoot + "/" + api
	switch api {
	case transport.Upload2AgentAPI:
		for _, sh := range network.FTPStreams {
			if token == sh.Token {
				ftp.HandleFTPTransfer(sh, wrt, req)
				return
			}
		}
		logging.Debugf("FTP stream not found: %s", token)
		wrt.WriteHeader(http.StatusNotFound)
	case transport.DownloadFile2AgentAPI:
		path := filepath.Clean(req.URL.Query().Get("file_to_download"))
		path = filepath.Base(path)
		logging.Infof("PUT: got request for file: %s, URL: %s", path, req.URL)
		local_path := fmt.Sprintf("%s%s", live.WWWRoot, path)
		if !util.IsExist(local_path) {
			logging.Warningf("File %s not found", local_path)
			wrt.WriteHeader(http.StatusNotFound)
			return
		}
		http.ServeFile(wrt, req, local_path)
	case transport.PortMappingAPI:
		network.HandlePortMapping(network.ProxyStream, wrt, req)
	default:
		logging.Debugf("API not found: %s", api)
		wrt.WriteHeader(http.StatusNotFound)
	}
}

// WgFileServer serves a file over HTTP on WireGuard interface
func WgFileServer(path_to_file string) (err error) {
	http.HandleFunc("/", func(wrt http.ResponseWriter, req *http.Request) {
		http.ServeFile(wrt, req, path_to_file)
	})
	return http.ListenAndServe(fmt.Sprintf("%s:%d", netutil.WgServerIP, netutil.WgFileServerPort), nil)
}
