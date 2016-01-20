package main

import (
	"fmt"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/fsouza/go-dockerclient"
)

type DockerContainerInfo struct {
	ContainerInfo
	RefreshTime time.Time
}

type DockerContainerService struct {
	containerIPMap map[string]DockerContainerInfo
	docker         *docker.Client
}

func NewDockerContainerService(docker *docker.Client) *DockerContainerService {
	return &DockerContainerService{
		containerIPMap: make(map[string]DockerContainerInfo),
		docker:         docker,
	}
}

func NewDockerContainerServiceFromConfig(config PlatformConfig) (*DockerContainerService, error) {
	endpoint := config.GetString("endpoint", "unix:///var/run/docker.sock")
	client, err := docker.NewClient(endpoint)

	if err != nil {
		return nil, err
	}

	return NewDockerContainerService(client), nil
}

func (self *DockerContainerService) TypeName() string {
	return "docker"
}

func (self *DockerContainerService) ContainerForIP(containerIP string) (ContainerInfo, error) {
	info, found := self.containerIPMap[containerIP]
	now := time.Now()

	if !found {
		self.syncContainers(now)
		info, found = self.containerIPMap[containerIP]
	} else if now.After(info.RefreshTime) {
		info, found = self.syncContainer(containerIP, info, now)
	}

	if !found {
		return ContainerInfo{}, fmt.Errorf("No container found for IP %s", containerIP)
	}

	return info.ContainerInfo, nil
}

func (self *DockerContainerService) syncContainer(containerIp string, oldInfo DockerContainerInfo, now time.Time) (DockerContainerInfo, bool) {
	log.Debug("Inspecting container: ", oldInfo.Id)
	_, err := self.docker.InspectContainer(oldInfo.Id)

	if err != nil {
		if _, ok := err.(*docker.NoSuchContainer); ok {
			log.Debug("Container not found, refreshing container info: ", oldInfo.Id)
		} else {
			log.Warn("Error inspecting container, refreshing container info: ", oldInfo.Id, ": ", err)
		}

		self.syncContainers(now)
		info, found := self.containerIPMap[containerIp]
		return info, found
	} else {
		oldInfo.RefreshTime = refreshTime(now)
		self.containerIPMap[containerIp] = oldInfo
		return oldInfo, true
	}
}

func (self *DockerContainerService) syncContainers(now time.Time) {
	log.Info("Synchronizing state with running docker containers")
	apiContainers, err := self.docker.ListContainers(docker.ListContainersOptions{
		All:    false, // only running containers
		Size:   false, // do not need size information
		Limit:  0,     // all running containers
		Since:  "",    // not applicable
		Before: "",    // not applicable
	})

	if err != nil {
		log.Error("Error listing running containers: ", err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]DockerContainerInfo)

	for _, apiContainer := range apiContainers {
		container, err := self.docker.InspectContainer(apiContainer.ID)

		if err != nil {
			if _, ok := err.(*docker.NoSuchContainer); ok {
				log.Debug("Container not found: ", apiContainer.ID)
			} else {
				log.Warn("Error inspecting container: ", apiContainer.ID, ": ", err)
			}

			continue
		}

		containerIP := container.NetworkSettings.IPAddress
		roleArn, iamPolicy, err := getRoleArnFromEnv(container.Config.Env)

		if err != nil {
			log.Error("Error getting role from container: ", apiContainer.ID, ": ", err)
			continue
		}

		log.Infof("Container: id=%s image=%s role=%s", container.ID[:6], container.Config.Image, roleArn)

		containerIPMap[containerIP] = DockerContainerInfo{
			ContainerInfo: ContainerInfo{
				Id:        container.ID,
				Name:      container.Name,
				IamRole:   roleArn,
				IamPolicy: iamPolicy,
			},
			RefreshTime: refreshAt,
		}
	}

	self.containerIPMap = containerIPMap
}

func refreshTime(now time.Time) time.Time {
	return now.Add(1 * time.Second)
}

func getRoleArnFromEnv(env []string) (role RoleArn, policy string, err error) {
	for _, e := range env {
		v := strings.SplitN(e, "=", 2)

		if v[0] == "IAM_ROLE" && len(v) > 1 {
			roleArn := strings.TrimSpace(v[1])

			if len(roleArn) > 0 {
				role, err = NewRoleArn(roleArn)

				if err != nil {
					return
				}
			}
		} else if v[0] == "IAM_POLICY" && len(v) > 1 {
			policy = strings.TrimSpace(v[1])
		}
	}

	return
}
