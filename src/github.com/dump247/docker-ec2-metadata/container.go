package main

type ContainerInfo struct {
	Id      string
	Name    string
	IamRole RoleArn
}

type ContainerService interface {
	ContainerForIP(containerIP string) (ContainerInfo, error)
}
