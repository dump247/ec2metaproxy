A service that runs on an EC2 instance that proxies the EC2 instance metadata service
for linux containers. The proxy overrides metadata endpoints for individual
containers.

The following container platforms are supported:

* [docker](https://www.docker.com)
* [flynn](https://flynn.io)

At this point, the only endpoint overridden is the security credentials. This allows
for different containers to have different IAM permissions and not just use the permissions
provided by the instance profile. However, this same technique could be used to override
any other endpoints where appropriate.

The proxy works by mapping the metadata source request IP to the container using the container
platform specific API. The container's metadata contains information about what IAM permissions
to use. Therefore, the proxy does not work for containers that do not use the container
network bridge (for example, containers using "host" networking).

# Setup

## Host

The host EC2 instance must have firewall settings that redirect any EC2 metadata connections
from containers to the metadata proxy. The proxy will then process the request and
may forward the request to the real metadata service.

The instance profile of the host EC2 instance must also have permission to assume the IAM roles
for the containers.

See:

* [Host Setup](docs/host-setup.md)

## Containers

Containers do not require any changes or modifications to utilize the metadata proxy. By
default, they will receive the default permissions configured by the proxy. Alternatively,
a container can be configured to use a separate IAM role or provide an IAM policy.

See:

* [Docker Container Setup](docs/docker-container-setup.md)
* [Flynn Container Setup](docs/flynn-container-setup.md)

# License

The MIT License (MIT)
Copyright (c) 2014 Cory Thomas

See [LICENSE](LICENSE)
