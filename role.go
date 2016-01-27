package main

import (
	"errors"
	"regexp"
	"time"
)

var (
	roleArnRegex = regexp.MustCompile(`^arn:aws:iam::(\d+):role/([^:]+/)?([^:]+?)$`)
)

type roleArn struct {
	value     string
	path      string
	name      string
	accountID string
}

func newRoleArn(value string) (roleArn, error) {
	result := roleArnRegex.FindStringSubmatch(value)

	if result == nil {
		return roleArn{}, errors.New("invalid role ARN")
	}

	return roleArn{value, "/" + result[2], result[3], result[1]}, nil
}

func (r roleArn) RoleName() string {
	return r.name
}

func (r roleArn) Path() string {
	return r.path
}

func (r roleArn) AccountID() string {
	return r.accountID
}

func (r roleArn) String() string {
	return r.value
}

func (r roleArn) Empty() bool {
	return len(r.value) == 0
}

func (r roleArn) Equals(other roleArn) bool {
	return r.value == other.value
}

type roleCredentials struct {
	AccessKey  string
	SecretKey  string
	Token      string
	Expiration time.Time
}

func (t *roleCredentials) ExpiredNow() bool {
	return t.ExpiredAt(time.Now())
}

func (t *roleCredentials) ExpiredAt(at time.Time) bool {
	return at.After(t.Expiration)
}
