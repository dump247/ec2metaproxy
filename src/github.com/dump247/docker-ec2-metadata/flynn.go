package main

import (
	"fmt"

	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/flynn/flynn/host/types"
	"github.com/flynn/flynn/pkg/cluster"
)

type FlynnContainerInfo struct {
	ContainerInfo
	RefreshTime time.Time
}

type FlynnContainerService struct {
	containerIPMap map[string]FlynnContainerInfo
	flynn          *cluster.Host
}

func NewFlynnContainerService(flynn *cluster.Host) *FlynnContainerService {
	return &FlynnContainerService{
		containerIPMap: make(map[string]FlynnContainerInfo),
		flynn:          flynn,
	}
}

func NewFlynnContainerServiceFromConfig(config PlatformConfig) (*FlynnContainerService, error) {
	endpoint := config.GetString("endpoint", "http://127.0.0.1:1113")
	flynn := cluster.NewHost("local", endpoint, nil, nil)
	return NewFlynnContainerService(flynn), nil
}

func (self *FlynnContainerService) TypeName() string {
	return "flynn"
}

func (self *FlynnContainerService) ContainerForIP(containerIP string) (ContainerInfo, error) {
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

func (self *FlynnContainerService) syncContainer(containerIp string, oldInfo FlynnContainerInfo, now time.Time) (FlynnContainerInfo, bool) {
	log.Debug("Inspecting job: ", oldInfo.Id)
	_, err := self.flynn.GetJob(oldInfo.Id)

	if err != nil {
		if err == cluster.ErrNotFound {
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

func (self *FlynnContainerService) syncContainers(now time.Time) {
	log.Info("Synchronizing state with running flynn containers")
	jobs, err := self.flynn.ListJobs()

	if err != nil {
		log.Error("Error listing running containers: ", err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]FlynnContainerInfo)

	for _, job := range jobs {
		roleArn, err := getRoleArnFromJob(job.Job)

		if err != nil {
			log.Error("Error getting role from container: ", job.ContainerID, ": ", err)
			continue
		}

		log.Infof("Job: id=%s role=%s", job.Job.ID, roleArn)

		containerIPMap[job.InternalIP] = FlynnContainerInfo{
			ContainerInfo: ContainerInfo{
				Id:        job.Job.ID,
				Name:      job.Job.ID,
				IamRole:   roleArn,
				IamPolicy: strings.TrimSpace(job.Job.Metadata["IAM_POLICY"]),
			},
			RefreshTime: refreshAt,
		}
	}

	self.containerIPMap = containerIPMap
}

func getRoleArnFromJob(job *host.Job) (RoleArn, error) {
	roleArn := job.Metadata["IAM_ROLE"]

	if len(roleArn) > 0 {
		return NewRoleArn(roleArn)
	}

	return RoleArn{}, nil
}
