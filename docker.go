package main

import (
	"fmt"
	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/fsouza/go-dockerclient"
)

type dockerContainerInfo struct {
	containerInfo
	RefreshTime time.Time
}

type dockerContainerService struct {
	containerIPMap map[string]dockerContainerInfo
	docker         *docker.Client
}

func newDockerContainerService(endpoint string) (*dockerContainerService, error) {
	client, err := docker.NewClient(endpoint)

	if err != nil {
		return nil, err
	}

	return &dockerContainerService{
		containerIPMap: make(map[string]dockerContainerInfo),
		docker:         client,
	}, nil
}

func (d *dockerContainerService) TypeName() string {
	return "docker"
}

func (d *dockerContainerService) ContainerForIP(containerIP string) (containerInfo, error) {
	info, found := d.containerIPMap[containerIP]
	now := time.Now()

	if !found {
		d.syncContainers(now)
		info, found = d.containerIPMap[containerIP]
	} else if now.After(info.RefreshTime) {
		info, found = d.syncContainer(containerIP, info, now)
	}

	if !found {
		return containerInfo{}, fmt.Errorf("No container found for IP %s", containerIP)
	}

	return info.containerInfo, nil
}

func (d *dockerContainerService) syncContainer(containerIP string, oldInfo dockerContainerInfo, now time.Time) (dockerContainerInfo, bool) {
	log.Debug("Inspecting container: ", oldInfo.ID)
	container, err := d.docker.InspectContainer(oldInfo.ID)

	if err != nil || !container.State.Running {
		if _, ok := err.(*docker.NoSuchContainer); ok {
			log.Debug("Container not found, refreshing container info: ", oldInfo.ID)
		} else {
			log.Warn("Error inspecting container, refreshing container info: ", oldInfo.ID, ": ", err)
		}

		d.syncContainers(now)
		info, found := d.containerIPMap[containerIP]
		return info, found
	}

	oldInfo.RefreshTime = refreshTime(now)
	d.containerIPMap[containerIP] = oldInfo
	return oldInfo, true
}

func (d *dockerContainerService) syncContainers(now time.Time) {
	log.Info("Synchronizing state with running docker containers")
	apiContainers, err := d.docker.ListContainers(docker.ListContainersOptions{
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
	containerIPMap := make(map[string]dockerContainerInfo)

	for _, apiContainer := range apiContainers {
		container, err := d.docker.InspectContainer(apiContainer.ID)

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

		containerIPMap[containerIP] = dockerContainerInfo{
			containerInfo: containerInfo{
				ID:        container.ID,
				Name:      container.Name,
				IamRole:   roleArn,
				IamPolicy: iamPolicy,
			},
			RefreshTime: refreshAt,
		}
	}

	d.containerIPMap = containerIPMap
}

func refreshTime(now time.Time) time.Time {
	return now.Add(1 * time.Second)
}

func getRoleArnFromEnv(env []string) (role roleArn, policy string, err error) {
	for _, e := range env {
		v := strings.SplitN(e, "=", 2)

		if v[0] == "IAM_ROLE" && len(v) > 1 {
			roleArn := strings.TrimSpace(v[1])

			if len(roleArn) > 0 {
				role, err = newRoleArn(roleArn)

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
