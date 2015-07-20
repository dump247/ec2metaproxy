A service that runs on an EC2 instance that proxies the EC2 instance metadata service
for docker containers. The proxy overrides metadata endpoints for individual docker
containers.

At this point, the only endpoint overridden is the security credentials. This allows
for different containers to have different IAM permissions and not just use the permissions
provided by the instance profile. However, this same technique could be used to override
any other endpoints where appropriate.

# Build

Requires:

* golang 1.2+
* make

The dependencies are managed with git submodules. After cloning, make sure to initialize the submodules:

```bash
git submodule init
git submodule update
```

Run `make` or `make build`. The resulting executable will be in `bin/`.

Run `make clean` to clear out any build artifacts.

## RPM

Requires:

* rpm-build

1. Run `make` to create the executable
2. Run `make rpm` to build the rpm

The steps can all be performed at once with `make build rpm` or `make clean build rpm`.

# Setup

What is needed is an EC2 instance with assume role permissions and one or more roles defined
that it can assume. The roles will be used by the containers to acquire their own permissions.

## Permissions

The EC2 instance must have permission to assume the roles required by the docker containers.

Note that assuming a role is two way: the permission to assume roles has to be granted to the
instance profile and the role has to allow the instance profile to assume it.

There must be at least one role to use as the default role if the container has not specified
one. The default role can have empty permissions (zero access) or some set of standard
permissions.

Example CloudFormation:

* _DockerContainerRole1_ is the set of permissions for a docker container
* _InstanceRole_ is the role used by the EC2 instance profile and needs permission to assume _DockerContainerRole1_

```json
"Resources": {
  "DockerContainerRole1": {
    "Type": "AWS::IAM::Role",
    "Properties": {
      "AssumeRolePolicyDocument": {
        "Statement": [
          {
            "Effect": "Allow",
            "Principal": {
              "AWS": [ {"Fn::GetAtt" : ["InstanceRole", "Arn"]} ]
            },
            "Action": [ "sts:AssumeRole" ]
          }
        ]
      },
      "Path": "/docker/",
      "Policies": []
    }
  },

  "InstanceRole": {
    "Type": "AWS::IAM::Role",
    "Properties": {
      "AssumeRolePolicyDocument": {
        "Statement": [
         {
           "Effect": "Allow",
           "Principal": {
              "Service": [ "ec2.amazonaws.com" ]
            },
            "Action": [ "sts:AssumeRole" ]
          }
        ]
      },
      "Path": "/",
      "Policies": [
        {
          "PolicyName": "AssumeRoles",
          "PolicyDocument": {
            "Statement": [
              {
                "Effect": "Allow",
                "Resource": {"Fn::Join": ["", ["arn:aws:iam::", {"Ref": "AWS::AccountId"}, ":role/docker/*"]]},
                "Action": [ "sts:AssumeRole" ]
              }
            ]
          }
        }
      ]
    }
  },

  "InstanceProfile": {
    "Type": "AWS::IAM::InstanceProfile",
    "Properties": {
      "Path": "/",
      "Roles": [{"Ref": "InstanceRole"}]
    }
  }
}
```

## Instance Setup

Install docker and run with the standard unix domain socket (_/var/run/docker.sock_).

Setup a rule to route all metadata service traffic from the containers
to the proxy service. By default, the proxy runs on _0.0.0.0:18000_, but this can
be overridden. Be careful that you do not expose the service outside the EC2 instance!

This script will setup the routing rules. You may need to adjust if you make changes
to how docker sets up its networking. If you are an iptables ninja and can improve this,
please submit a ticket or a pull request!

```bash
# Get the host IP address. You can use a different mechanism if you wish.
# Note that IP can not be 127.0.0.1 because DNAT for loopback is not possible.
PROXY_IP=$(ifconfig eth0 | grep -Eo "inet addr:[0-9.]+" | grep -Eo "[0-9.]+")

# Port that the proxy service runs on. Default is 18000.
PROXY_PORT=18000

# Drop any traffic to the proxy service that is NOT coming from docker containers
iptables                    \
    -I INPUT                \
    -p tcp                  \
    --dport ${PROXY_PORT}   \
    ! -i docker0            \
    -j DROP

# Redirect any requests from docker containers to the proxy service
iptables                                        \
    -t nat                                      \
    -I PREROUTING                               \
    -p tcp                                      \
    -d 169.254.169.254 --dport 80               \
    -j DNAT                                     \
    --to-destination ${PROXY_IP}:${PROXY_PORT}  \
    -i docker0
```

## Run the Service

The proxy service will need to run as root or as a user in the _docker_ group.

# Container Role

If the container does not specify a role, the default role is used. A container can specify
a specific role to use by setting the `IAM_ROLE` environment variable.

Example: `IAM_ROLE=arn:aws:iam::123456789012:role/CONTAINER_ROLE_NAME`

Note that the host machine’s instance profile must have permission to assume the given role.
If not, the container will receive an error when requesting the credentials.

# License

The MIT License (MIT)
Copyright (c) 2014 Cory Thomas

See [LICENSE](LICENSE)
