package main

import (
	"fmt"
)

type ContainerInfo struct {
	Id        string
	Name      string
	IamRole   RoleArn
	IamPolicy string
}

type ContainerService interface {
	ContainerForIP(containerIP string) (ContainerInfo, error)
	TypeName() string
}

func NewContainerService(config PlatformConfig) (ContainerService, error) {
	platformType := config["type"]

	if platformType == "docker" {
		return NewDockerContainerServiceFromConfig(config)
	} else if platformType == "flynn" {
		return NewFlynnContainerServiceFromConfig(config)
	} else {
		return nil, fmt.Errorf("Unknown platform type: %s", platformType)
	}

	return nil, nil
}
