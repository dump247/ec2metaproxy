package main

type containerInfo struct {
	ID        string
	Name      string
	IamRole   roleArn
	IamPolicy string
}

type containerService interface {
	ContainerForIP(containerIP string) (containerInfo, error)
	TypeName() string
}
