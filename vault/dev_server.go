package vault

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/finleap-connect/vaultoperator/util"
)

type DevServer struct {
	expectShutdown int32 // atomic variable to check shutdown state
	ctx            context.Context
	cancelFunc     context.CancelFunc
	addr           string
	rootToken      string
	vaultBin       string
}

func NewDevServer() (*DevServer, error) {
	port, err := util.RandomPort()
	if err != nil {
		return nil, err
	}
	bin := "vault"
	if value, ok := os.LookupEnv("VAULT"); ok {
		bin = value
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv := &DevServer{expectShutdown: 0, ctx: ctx, cancelFunc: cancel, addr: "127.0.0.1:" + port, rootToken: "root", vaultBin: bin}
	cmd := exec.CommandContext(ctx, srv.vaultBin, "server", "-dev", "-dev-listen-address", srv.addr, "-dev-root-token-id", "root")
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go func() {
		err := cmd.Wait()
		if err != nil && atomic.LoadInt32(&srv.expectShutdown) == 0 {
			panic(fmt.Sprintf("vault server -dev failed this is most likely because vault CLI was not found: %v", err))
		}
	}()
	return srv, nil
}

func (s *DevServer) GetClient(namespace string) (*Client, error) {
	return NewClient("http://"+s.addr, namespace, &TokenAuth{Token: s.rootToken})
}

func (s *DevServer) Stop() error {
	swapped := atomic.CompareAndSwapInt32(&s.expectShutdown, 0, 1)
	if !swapped {
		panic("shutdown flag swap failed")
	}
	if s.ctx.Err() == nil {
		s.cancelFunc()
	}
	return nil
}

func (s *DevServer) ExecCommand(arg ...string) error {
	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 10 * time.Second
	return backoff.Retry(func() error {
		cmd := exec.Command(s.vaultBin, arg...)
		cmd.Env = append(os.Environ(), "VAULT_ADDR=http://"+s.addr, "VAULT_TOKEN="+s.rootToken)
		return cmd.Run()
	}, bo)
}
