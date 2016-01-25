package main

type ContainerInfo struct {
	Id        string
	Name      string
	IamRole   RoleArn
	IamPolicy string
}

type ContainerService interface {
	ContainerForIP(containerIP string) (ContainerInfo, error)
	TypeName() string
}
