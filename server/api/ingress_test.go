package api_test

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/log"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/server/api"
	"github.com/cosmos/cosmos-sdk/server/config"
)

// REST handlers must receive the original reader, including its deadline and
// size errors, rather than a body buffered by JSON-RPC batch preprocessing.
func TestRESTHandlerOwnsBoundedBodyRead(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	address := listener.Addr().String()
	require.NoError(t, listener.Close())
	server := api.New(client.Context{}, log.NewNopLogger(), nil)
	entered := make(chan struct{}, 2)
	server.Router.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		_, readErr := io.ReadAll(r.Body)
		if readErr != nil {
			http.Error(w, readErr.Error(), http.StatusRequestTimeout)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}).Methods("POST")
	cfg := config.DefaultConfig()
	cfg.API.Address = "tcp://" + address
	cfg.API.RPCReadTimeout = 1
	cfg.API.RPCWriteTimeout = 5
	cfg.API.RPCMaxBodyBytes = 64
	cfg.GRPC.Enable = false
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Start(ctx, *cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("API server failed to stop")
		}
	})
	var conn net.Conn
	require.Eventually(t, func() bool { conn, err = net.DialTimeout("tcp", address, 50*time.Millisecond); return err == nil }, 5*time.Second, 10*time.Millisecond)
	defer conn.Close()
	require.NoError(t, conn.SetDeadline(time.Now().Add(4*time.Second)))
	_, err = fmt.Fprint(conn, "POST /upload HTTP/1.1\r\nHost: localhost\r\nContent-Length: 32\r\nConnection: close\r\n\r\n{")
	require.NoError(t, err)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("REST router did not receive the incomplete upload")
	}
	response, err := http.ReadResponse(bufio.NewReader(conn), nil)
	require.NoError(t, err)
	defer response.Body.Close()
	failure, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusRequestTimeout, response.StatusCode)
	require.Contains(t, string(failure), "timeout")

	boundedClient := &http.Client{Timeout: 3 * time.Second}
	oversized, err := boundedClient.Post("http://"+address+"/upload", "application/json", strings.NewReader(strings.Repeat("x", 100)))
	require.NoError(t, err)
	defer oversized.Body.Close()
	failure, err = io.ReadAll(oversized.Body)
	require.NoError(t, err)
	require.Contains(t, string(failure), "request body too large")
}
