package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/moby/moby/client"
)

type ContainerInfo struct {
	ID      string
	Names   string
	Image   string
	Status  string
	State   string
	Ports   string
	Created time.Time
}

func GetContainers(all bool) ([]ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		return nil, fmt.Errorf("Docker daemon not reachable: %w", err)
	}
	defer apiClient.Close()

	result, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: all,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	var list []ContainerInfo
	for _, ctr := range result.Items {
		shortID := ctr.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}

		names := strings.Join(ctr.Names, ", ")
		names = strings.TrimPrefix(names, "/")

		var portStrs []string
		for _, p := range ctr.Ports {
			if p.PublicPort != 0 {
				portStrs = append(portStrs, fmt.Sprintf("%d->%d/%s", p.PublicPort, p.PrivatePort, p.Type))
			} else {
				portStrs = append(portStrs, fmt.Sprintf("%d/%s", p.PrivatePort, p.Type))
			}
		}

		list = append(list, ContainerInfo{
			ID:      shortID,
			Names:   names,
			Image:   ctr.Image,
			Status:  ctr.Status,
			State:   string(ctr.State),
			Ports:   strings.Join(portStrs, ", "),
			Created: time.Unix(ctr.Created, 0),
		})
	}

	return list, nil
}
