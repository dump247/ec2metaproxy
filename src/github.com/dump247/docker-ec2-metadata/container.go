package main

import (
	"fmt"
)

type ContainerInfo struct {
	Id      string
	Name    string
	IamRole RoleArn
}

type ContainerService interface {
	ContainerForIP(containerIP string) (ContainerInfo, error)
}

func NewContainerService(config PlatformConfig) (ContainerService, error) {
	platformType := config["type"]

	if platformType == "docker" {
		return NewDockerContainerServiceFromConfig(config)
	} else {
		return nil, fmt.Errorf("Unknown platform type: %s", platformType)
	}

	return nil, nil
}
