// Package mobile is the gomobile binding used by the Android app. It starts
// the Career Assessment 107 runtime and web UI on a loopback port inside the
// app process; the app's WebView loads the returned URL. Nothing listens on a
// network interface, and nothing leaves the device.
package mobile

import (
	"fmt"
	"net"
	"net/http"
	"sync"

	"career107/api"
)

var (
	mu      sync.Mutex
	baseURL string
)

// Start launches the local server (once per process) and returns its base
// URL. dataDir is the app's private files directory, where the session is
// saved so it survives the app being closed.
func Start(dataDir string) (string, error) {
	mu.Lock()
	defer mu.Unlock()
	if baseURL != "" {
		return baseURL, nil
	}
	session, err := api.NewSession(dataDir)
	if err != nil {
		return "", err
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	go func() { _ = http.Serve(listener, api.NewHandler(session)) }()
	baseURL = fmt.Sprintf("http://%s/", listener.Addr().String())
	return baseURL, nil
}
