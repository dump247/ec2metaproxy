package main

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNewRoleArn(t *testing.T) {
	assert := assert.New(t)

	arn, err := NewRoleArn("arn:aws:iam::123456789012:role/test-role-name")
	assert.Nil(err)
	assert.Equal("test-role-name", arn.RoleName())
	assert.Equal("/", arn.Path())
	assert.Equal("123456789012", arn.AccountId())
	assert.Equal("arn:aws:iam::123456789012:role/test-role-name", arn.String())
}

func TestNewRoleArnWithPath(t *testing.T) {
	assert := assert.New(t)

	arn, err := NewRoleArn("arn:aws:iam::123456789012:role/this/is/the/path/test-role-name")
	assert.Nil(err)
	assert.Equal("test-role-name", arn.RoleName())
	assert.Equal("/this/is/the/path/", arn.Path())
	assert.Equal("123456789012", arn.AccountId())
	assert.Equal("arn:aws:iam::123456789012:role/this/is/the/path/test-role-name", arn.String())
}
