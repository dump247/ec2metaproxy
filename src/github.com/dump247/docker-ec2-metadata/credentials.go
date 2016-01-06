package main

import (
	"fmt"
	"github.com/goamz/goamz/aws"
	"github.com/goamz/goamz/sts"
	"regexp"
	"sync"
	"time"
)

const (
	maxSessionNameLen int = 32
)

var (
	// matches char that is not valid in a STS role session name
	invalidSessionNameRegexp *regexp.Regexp = regexp.MustCompile(`[^\w+=,.@-]`)
)

type Credentials struct {
	AccessKey   string
	Expiration  time.Time
	GeneratedAt time.Time
	RoleArn     RoleArn
	SecretKey   string
	Token       string
}

func (self Credentials) ExpiredNow() bool {
	return self.ExpiredAt(time.Now())
}

func (self Credentials) ExpiredAt(at time.Time) bool {
	return at.After(self.Expiration)
}

func (self Credentials) ExpiresIn(d time.Duration) bool {
	return self.ExpiredAt(time.Now().Add(-d))
}

type ContainerCredentials struct {
	ContainerInfo
	Credentials
}

func (self ContainerCredentials) IsValid(container ContainerInfo) bool {
	return self.ContainerInfo.IamRole.Equals(container.IamRole) &&
		self.ContainerInfo.Id == container.Id &&
		!self.Credentials.ExpiresIn(5*time.Minute)
}

type CredentialsProvider struct {
	container            ContainerService
	auth                 aws.Auth
	defaultIamRoleArn    RoleArn
	containerCredentials map[string]ContainerCredentials
	lock                 sync.Mutex
}

func NewCredentialsProvider(auth aws.Auth, container ContainerService, defaultIamRoleArn RoleArn) *CredentialsProvider {
	return &CredentialsProvider{
		container:            container,
		auth:                 auth,
		defaultIamRoleArn:    defaultIamRoleArn,
		containerCredentials: make(map[string]ContainerCredentials),
	}
}

func (self *CredentialsProvider) CredentialsForIP(containerIP string) (Credentials, error) {
	self.lock.Lock()
	defer self.lock.Unlock()

	container, err := self.container.ContainerForIP(containerIP)

	if err != nil {
		return Credentials{}, err
	}

	oldCredentials, found := self.containerCredentials[containerIP]

	if !found || !oldCredentials.IsValid(container) {
		roleArn := container.IamRole

		if roleArn.Empty() {
			roleArn = self.defaultIamRoleArn
		}

		role, err := self.AssumeRole(roleArn, generateSessionName(container.Id))

		if err != nil {
			return Credentials{}, err
		}

		oldCredentials = ContainerCredentials{container, role}
		self.containerCredentials[containerIP] = oldCredentials
	}

	return oldCredentials.Credentials, nil
}

func (self *CredentialsProvider) AssumeRole(roleArn RoleArn, sessionName string) (Credentials, error) {
	stsClient := sts.New(self.auth, aws.USEast)
	resp, err := stsClient.AssumeRole(&sts.AssumeRoleParams{
		DurationSeconds: 3600, // Max is 1 hour
		ExternalId:      "",   // Empty string means not applicable
		Policy:          "",   // Empty string means not applicable
		RoleArn:         roleArn.String(),
		RoleSessionName: sessionName,
	})

	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		AccessKey:   resp.Credentials.AccessKeyId,
		SecretKey:   resp.Credentials.SecretAccessKey,
		Token:       resp.Credentials.SessionToken,
		Expiration:  resp.Credentials.Expiration,
		GeneratedAt: time.Now(),
	}, nil
}

func generateSessionName(containerId string) string {
	sessionName := fmt.Sprintf("metadata-proxy-%s", containerId)
	return invalidSessionNameRegexp.ReplaceAllString(sessionName, "_")[0:maxSessionNameLen]
}
