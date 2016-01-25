package main

import (
	"github.com/alecthomas/kingpin"
)

type RoleArnValue RoleArn

func (self *RoleArnValue) Set(value string) error {
	if len(value) > 0 {
		arn, err := NewRoleArn(value)
		*(*RoleArn)(self) = arn
		return err
	}

	return nil
}

func (t *RoleArnValue) String() string {
	return ""
}

func RoleArnOpt(s kingpin.Settings) (target *RoleArn) {
	target = new(RoleArn)
	s.SetValue((*RoleArnValue)(target))
	return
}
