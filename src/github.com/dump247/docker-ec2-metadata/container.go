package main

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/fsouza/go-dockerclient"
	"github.com/goamz/goamz/aws"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxSessionNameLen        int           = 32
	credentialsRefreshPeriod time.Duration = 30 * time.Second
)

var (
	// matches char that is not valid in a STS role session name
	invalidSessionNameRegexp *regexp.Regexp = regexp.MustCompile(`[^\w+=,.@-]`)
)

type ContainerInfo struct {
	ContainerId      string
	ShortContainerId string
	SessionName      string
	LastUpdated      time.Time
	Error            error
	RoleArn          RoleArn
	Credentials      *RoleCredentials
}

func (t *ContainerInfo) RequiresRefresh() bool {
	if t.RoleArn.Empty() {
		return false
	}

	if t.Credentials != nil {
		return t.Credentials.ExpiredNow()
	}

	return time.Since(t.LastUpdated) > credentialsRefreshPeriod
}

type ContainerRole struct {
	LastUpdated time.Time
	Arn         RoleArn
	Credentials *RoleCredentials
}

type ContainerService struct {
	containerIPMap map[string]*ContainerInfo
	containerIdMap map[string]string // container id => container IP
	docker         *docker.Client
	defaultRoleArn RoleArn
	auth           aws.Auth
	lock           sync.Mutex
}

func NewContainerService(docker *docker.Client, defaultRoleArn RoleArn, auth aws.Auth) *ContainerService {
	return &ContainerService{
		containerIPMap: make(map[string]*ContainerInfo),
		containerIdMap: make(map[string]string),
		docker:         docker,
		defaultRoleArn: defaultRoleArn,
		auth:           auth,
	}
}

func (t *ContainerService) RoleForIP(containerIP string) (*ContainerRole, error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	info, err := t.containerForIP(containerIP)

	if err != nil {
		return nil, err
	}

	if info.RequiresRefresh() {
		log.Infof("Refreshing role for container %s: role=%s session=%s", info.ShortContainerId, info.RoleArn, info.SessionName)
		creds, err := AssumeRole(t.auth, info.RoleArn.String(), info.SessionName)

		info.LastUpdated = time.Now()
		info.Error = err
		info.Credentials = creds
	}

	if info.Error != nil {
		return nil, info.Error
	}

	return &ContainerRole{info.LastUpdated, info.RoleArn, info.Credentials}, nil
}

func (t *ContainerService) containerForIP(containerIP string) (*ContainerInfo, error) {
	info, found := t.containerIPMap[containerIP]

	if !found {
		t.syncContainers()
		info, found = t.containerIPMap[containerIP]

		if !found {
			return nil, fmt.Errorf("No container found for IP %s", containerIP)
		}
	}

	return info, nil
}

func (t *ContainerService) syncContainers() {
	log.Info("Synchronizing state with running docker containers")
	apiContainers, err := t.docker.ListContainers(docker.ListContainersOptions{
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

	containerIPMap := make(map[string]*ContainerInfo)
	containerIdMap := make(map[string]string)

	for _, apiContainer := range apiContainers {
		container, err := t.docker.InspectContainer(apiContainer.ID)

		if err != nil {
			log.Error("Error inspecting container: ", apiContainer.ID, ": ", err)
			continue
		}

		shortContainerId := apiContainer.ID[:6]
		containerIP := container.NetworkSettings.IPAddress

		roleArn, roleErr := getRoleArnFromEnv(container.Config.Env, t.defaultRoleArn)

		if roleArn.Empty() && roleErr == nil {
			roleErr = fmt.Errorf("No role defined for container %s: image=%s", shortContainerId, container.Config.Image)
		}

		log.Infof("Container: id=%s image=%s role=%s", shortContainerId, container.Config.Image, roleArn)

		containerIPMap[containerIP] = &ContainerInfo{
			ContainerId:      apiContainer.ID,
			ShortContainerId: shortContainerId,
			SessionName:      generateSessionName(container),
			LastUpdated:      time.Time{},
			Error:            roleErr,
			RoleArn:          roleArn,
		}

		containerIdMap[apiContainer.ID] = containerIP
	}

	t.containerIPMap = containerIPMap
	t.containerIdMap = containerIdMap
}

func getRoleArnFromEnv(env []string, defaultArn RoleArn) (RoleArn, error) {
	for _, e := range env {
		v := strings.SplitN(e, "=", 2)

		if len(v) > 1 && v[0] == "IAM_ROLE" && len(v[1]) > 0 {
			return NewRoleArn(v[1])
		}
	}

	return defaultArn, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func generateSessionName(container *docker.Container) string {
	containerId := container.ID[:6]

	remaining := maxSessionNameLen - (len(containerId) + 2) // 2 chars for separators
	containerName := container.Name[1:]                     // Strip '/' prefix
	imageName := container.Config.Image

	// Split the remaining number of characters between container and image name.
	// If one or the other is shorter than half the remaining, give the available
	// chars to the other string.

	// Trim container name
	containerNameLen := remaining / 2
	containerNameLen = (containerNameLen + maxInt(0, containerNameLen-len(imageName)))

	if containerNameLen < len(containerName) {
		containerName = containerName[len(containerName)-containerNameLen:]
	}

	// Trim image name
	imageNameLen := (remaining + 1) / 2 // If odd, image name gets the extra char
	imageNameLen = (imageNameLen + maxInt(0, imageNameLen-len(containerName)))

	if imageNameLen < len(imageName) {
		imageName = imageName[len(imageName)-imageNameLen:]
	}

	return invalidSessionNameRegexp.ReplaceAllString(fmt.Sprintf("%s-%s-%s", imageName, containerName, containerId), "_")
}
