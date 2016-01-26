Setting up the EC2 host requires three things:

1. Proper IAM permissions in the instance profile
2. Firewall settings to redirect metadata requests from containers
3. Metadata proxy service running

# Host IAM Permissions

The host EC2 instance must have permission to assume the roles required by the containers.

Note that assuming a role is two way: the permission to assume roles has to be granted to the
instance profile and the role has to allow the instance profile to assume it.

There must be at least one role to use as the default role if the container has not specified
one. The default role can have empty permissions (zero access) or some set of standard
permissions.

Example CloudFormation:

* _InstanceRole_ is the role used by the EC2 instance profile and needs permission to assume other roles
  * Note that, in this example, the AssumeRole permission is limited to roles with the path
    _/containers/_. While it is not required, a role path provides a small security
    benefit as it limits the instance to specific roles.
* _DefaultRole_ is the default role if the container does not specify a role
  * This role is set in the metadata proxy configuration
* _ContainerRole1_ is a custom set of permissions for a docker container
  * This role is set in the container metadata
  * This role could have completely different permissions from the default role

```json
{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Resources": {
    "ContainerRole1": {
      "Type": "AWS::IAM::Role",
      "Properties": {
        "AssumeRolePolicyDocument": {
          "Statement": [
            {
              "Effect": "Allow",
              "Principal": {
                "AWS": [{ "Fn::GetAtt" : ["InstanceRole", "Arn"] }]
              },
              "Action": ["sts:AssumeRole"]
            }
          ]
        },
        "Path": "/containers/",
        "Policies": [
          {
            "PolicyName": "DoAnythingEC2",
            "PolicyDocument": {
              "Statement": [
                {
                  "Effect": "Allow",
                  "Resource": "*",
                  "Action": [ "ec2:*" ]
                }
              ]
            }
          }
        ]
      }
    },

    "DefaultRole":{
      "Type": "AWS::IAM::Role",
      "Properties": {
        "AssumeRolePolicyDocument": {
          "Statement": [
            {
              "Effect": "Allow",
              "Principal": {
                "AWS": [{ "Fn::GetAtt" : ["InstanceRole", "Arn"] }]
              },
              "Action": ["sts:AssumeRole"]
            }
          ]
        },
        "Path": "/containers/",
        "Policies": [
          {
            "PolicyName": "DescribeEC2",
            "PolicyDocument": {
              "Statement": [
                {
                  "Effect": "Allow",
                  "Resource": "*",
                  "Action": [ "ec2:Describe*" ]
                }
              ]
            }
          }
        ]
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
                  "Resource": {"Fn::Join": ["", ["arn:aws:iam::", {"Ref": "AWS::AccountId"}, ":role/containers/*"]]},
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
}
```

# Firewall Settings

The idea is to redirect any connections to the standard EC2 metadata service IP that
originate from containers to the metadata proxy. This is accomplished by using
iptables to re-route any packets from the container network bridge to the metadata
IP to the proxy service.

A [shell script](../scripts/setup-firewall.sh) is available that sets up the firewall rules.

Making the changes persistent across reboots is system dependent.

```shell
./setup-firewall.sh --container-iface docker0
```

# Run Proxy Service

How to start the proxy service depends on the container system in use.

## Docker

A docker pre-built docker image is available that runs the metadata proxy.
The container needs host networking, otherwise it will attempt to connect
to itself when accessing the real metadata service. It also needs access
to the host docker domain socket so that it can get information about
running containers.

A [shell script](../scripts/run-docker.sh) is available that runs the
metadata proxy in a docker container. The script sets up the container
to auto-restart and run as a daemon.

```bash
./run-docker.sh --default-iam-role "arn:aws:iam::123456789012:role/DefaultRole"
```

## Flynn

TODO
