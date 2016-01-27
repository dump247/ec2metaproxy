package main

import (
	"fmt"

	"strings"
	"time"

	log "github.com/cihub/seelog"
	"github.com/flynn/flynn/host/types"
	"github.com/flynn/flynn/pkg/cluster"
)

type flynnContainerInfo struct {
	containerInfo
	RefreshTime time.Time
}

type flynnContainerService struct {
	containerIPMap map[string]flynnContainerInfo
	flynn          *cluster.Host
}

func newFlynnContainerService(endpoint string) (*flynnContainerService, error) {
	flynn := cluster.NewHost("local", endpoint, nil, nil)

	return &flynnContainerService{
		containerIPMap: make(map[string]flynnContainerInfo),
		flynn:          flynn,
	}, nil
}

func (f *flynnContainerService) TypeName() string {
	return "flynn"
}

func (f *flynnContainerService) ContainerForIP(containerIP string) (containerInfo, error) {
	info, found := f.containerIPMap[containerIP]
	now := time.Now()

	if !found {
		f.syncContainers(now)
		info, found = f.containerIPMap[containerIP]
	} else if now.After(info.RefreshTime) {
		info, found = f.syncContainer(containerIP, info, now)
	}

	if !found {
		return containerInfo{}, fmt.Errorf("No container found for IP %s", containerIP)
	}

	return info.containerInfo, nil
}

func (f *flynnContainerService) syncContainer(containerIP string, oldInfo flynnContainerInfo, now time.Time) (flynnContainerInfo, bool) {
	log.Debug("Inspecting job: ", oldInfo.ID)
	_, err := f.flynn.GetJob(oldInfo.ID)

	if err != nil {
		if err == cluster.ErrNotFound {
			log.Debug("Container not found, refreshing container info: ", oldInfo.ID)
		} else {
			log.Warn("Error inspecting container, refreshing container info: ", oldInfo.ID, ": ", err)
		}

		f.syncContainers(now)
		info, found := f.containerIPMap[containerIP]
		return info, found
	}

	oldInfo.RefreshTime = refreshTime(now)
	f.containerIPMap[containerIP] = oldInfo
	return oldInfo, true
}

func (f *flynnContainerService) syncContainers(now time.Time) {
	log.Info("Synchronizing state with running flynn containers")
	jobs, err := f.flynn.ListJobs()

	if err != nil {
		log.Error("Error listing running containers: ", err)
		return
	}

	refreshAt := refreshTime(now)
	containerIPMap := make(map[string]flynnContainerInfo)

	for _, job := range jobs {
		roleArn, err := getRoleArnFromJob(job.Job)

		if err != nil {
			log.Error("Error getting role from container: ", job.ContainerID, ": ", err)
			continue
		}

		log.Infof("Job: id=%s role=%s", job.Job.ID, roleArn)

		containerIPMap[job.InternalIP] = flynnContainerInfo{
			containerInfo: containerInfo{
				ID:        job.Job.ID,
				Name:      job.Job.ID,
				IamRole:   roleArn,
				IamPolicy: strings.TrimSpace(job.Job.Metadata["IAM_POLICY"]),
			},
			RefreshTime: refreshAt,
		}
	}

	f.containerIPMap = containerIPMap
}

func getRoleArnFromJob(job *host.Job) (roleArn, error) {
	roleArnStr := job.Metadata["IAM_ROLE"]

	if len(roleArnStr) > 0 {
		return newRoleArn(roleArnStr)
	}

	return roleArn{}, nil
}
