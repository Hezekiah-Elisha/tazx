//go:build linux

package network

import (
	"context"
	"os"
)

func getDefaultGateway(_ context.Context) (*GatewayInfo, error) {
	f, err := os.Open("/proc/net/route")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return ParseProcNetRoute(f)
}
