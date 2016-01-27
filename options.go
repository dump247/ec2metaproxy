package main

import (
	"github.com/alecthomas/kingpin"
)

type roleArnValue roleArn

func (r *roleArnValue) Set(value string) error {
	if len(value) > 0 {
		arn, err := newRoleArn(value)
		*(*roleArn)(r) = arn
		return err
	}

	return nil
}

func (r *roleArnValue) String() string {
	return ""
}

func roleArnOpt(s kingpin.Settings) (target *roleArn) {
	target = new(roleArn)
	s.SetValue((*roleArnValue)(target))
	return
}
