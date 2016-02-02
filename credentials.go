package main

import (
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sts"
)

const (
	maxSessionNameLen int = 32
)

var (
	// matches char that is not valid in a STS role session name
	invalidSessionNameRegexp = regexp.MustCompile(`[^\w+=,.@-]`)
)

type credentials struct {
	AccessKey   string
	Expiration  time.Time
	GeneratedAt time.Time
	RoleArn     roleArn
	SecretKey   string
	Token       string
}

func (c credentials) ExpiredNow() bool {
	return c.ExpiredAt(time.Now())
}

func (c credentials) ExpiredAt(at time.Time) bool {
	return at.After(c.Expiration)
}

func (c credentials) ExpiresIn(d time.Duration) bool {
	return c.ExpiredAt(time.Now().Add(-d))
}

type containerCredentials struct {
	containerInfo
	credentials
}

func (c containerCredentials) IsValid(container containerInfo) bool {
	return c.containerInfo.IamRole.Equals(container.IamRole) &&
		c.containerInfo.ID == container.ID &&
		!c.credentials.ExpiresIn(5*time.Minute)
}

type credentialsProvider struct {
	container            containerService
	awsSts               *sts.STS
	defaultIamRoleArn    roleArn
	defaultIamPolicy     string
	containerCredentials map[string]containerCredentials
	lock                 sync.Mutex
}

func newCredentialsProvider(awsSession *session.Session, container containerService, defaultIamRoleArn roleArn, defaultIamPolicy string) *credentialsProvider {
	return &credentialsProvider{
		container:            container,
		awsSts:               sts.New(awsSession),
		defaultIamRoleArn:    defaultIamRoleArn,
		defaultIamPolicy:     defaultIamPolicy,
		containerCredentials: make(map[string]containerCredentials),
	}
}

func (c *credentialsProvider) CredentialsForIP(containerIP string) (credentials, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	container, err := c.container.ContainerForIP(containerIP)

	if err != nil {
		return credentials{}, err
	}

	oldCredentials, found := c.containerCredentials[containerIP]

	if !found || !oldCredentials.IsValid(container) {
		roleArn := container.IamRole
		iamPolicy := container.IamPolicy

		if roleArn.Empty() {
			roleArn = c.defaultIamRoleArn

			if len(iamPolicy) == 0 {
				iamPolicy = c.defaultIamPolicy
			}
		}

		role, err := c.AssumeRole(roleArn, iamPolicy, generateSessionName(c.container.TypeName(), container.ID))

		if err != nil {
			return credentials{}, err
		}

		oldCredentials = containerCredentials{container, role}
		c.containerCredentials[containerIP] = oldCredentials
	}

	return oldCredentials.credentials, nil
}

func (c *credentialsProvider) AssumeRole(roleArn roleArn, iamPolicy, sessionName string) (credentials, error) {
	var policy *string

	if len(iamPolicy) > 0 {
		policy = aws.String(iamPolicy)
	}

	resp, err := c.awsSts.AssumeRole(&sts.AssumeRoleInput{
		DurationSeconds: aws.Int64(3600), // Max is 1 hour
		Policy:          policy,
		RoleArn:         aws.String(roleArn.String()),
		RoleSessionName: aws.String(sessionName),
	})

	if err != nil {
		return credentials{}, err
	}

	return credentials{
		AccessKey:   *resp.Credentials.AccessKeyId,
		SecretKey:   *resp.Credentials.SecretAccessKey,
		Token:       *resp.Credentials.SessionToken,
		Expiration:  *resp.Credentials.Expiration,
		GeneratedAt: time.Now(),
		RoleArn:     roleArn,
	}, nil
}

func generateSessionName(platform, containerID string) string {
	sessionName := fmt.Sprintf("%s-%s", platform, containerID)
	return invalidSessionNameRegexp.ReplaceAllString(sessionName, "_")[0:maxSessionNameLen]
}
