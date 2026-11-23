//go:build !linux

package network

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

func getDefaultGateway(ctx context.Context) (*GatewayInfo, error) {
	if runtime.GOOS != "darwin" {
		return nil, nil
	}
	runCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	out, err := exec.CommandContext(runCtx, "route", "-n", "get", "default").Output()
	if err != nil {
		return nil, err
	}
	gw := &GatewayInfo{}
	for _, raw := range strings.Split(string(out), "\n") {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "gateway:") {
			gw.GatewayIP = strings.TrimSpace(strings.TrimPrefix(line, "gateway:"))
		} else if strings.HasPrefix(line, "interface:") {
			gw.Interface = strings.TrimSpace(strings.TrimPrefix(line, "interface:"))
		}
	}
	if gw.GatewayIP == "" {
		return nil, nil
	}
	return gw, nil
}
