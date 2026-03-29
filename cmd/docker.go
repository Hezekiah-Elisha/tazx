package cmd

import (
	"context"
	"fmt"

	"github.com/moby/moby/client"
	"github.com/spf13/cobra"
)

var dockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Check the status of your Docker containers",
	Long:  `Get a quick overview of your Docker containers' health and performance.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Placeholder for docker command logic
		dockerFunction()
	},
}

func dockerFunction() {
	// Placeholder for Docker command logic
	ctx := context.Background()
	apiClient, err := client.New(client.FromEnv)
	if err != nil {
		// panic("Error occurred while creating Docker API client: " + err.Error())
		fmt.Print("It seems like Docker is not running or not properly configured. Please ensure Docker is installed and running on your system.")
		return
	}
	defer apiClient.Close()

	// List all containers (both stopped and running).
	result, err := apiClient.ContainerList(ctx, client.ContainerListOptions{
		All: true,
	})
	if err != nil {
		// panic("Error occurred while listing containers: " + err.Error())
		fmt.Print("Unable to retrieve Docker containers. Please check your Docker configuration and ensure it is running.")
		return
	}

	// Print each container's ID, status and the image it was created from.
	fmt.Printf("%s  %-22s  %s\n", "ID", "STATUS", "IMAGE")
	for _, ctr := range result.Items {
		fmt.Printf("%s  %-22s  %s\n", ctr.ID, ctr.Status, ctr.Image)
	}
}
