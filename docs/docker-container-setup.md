By default, a container receives the default permissions configured by the metadata
proxy. However, some containers require more or less permissions. For these cases, the
container can provide a custom IAM role, a custom IAM policy, or both.

The role and/or policy are configured in container environment variables. The metadata
proxy daemon uses the docker API to get the configured role and policy for a container.
The environment variable can only be set when the container is created and can not be
modified while the container is running.

# Container Role

A container can specify a specific role to use by setting the `IAM_ROLE` environment
variable on the image or the container. The metadata proxy will return credentials
for the given role when requested.

Example:

```bash
docker run -e 'IAM_ROLE=arn:aws:iam::123456789012:role/ContainerRoleName' ...
```

Note that the host machine’s instance profile must have permission to assume the given role.
If not, the container will receive an error when requesting the credentials.

# Container Policy

A container can specify a custom IAM policy by setting the `IAM_POLICY` environment
variable on the image or the container. The resulting container permissions will be
the intersection of the custom policy and the default container role or the role
specified by the container's `IAM_ROLE` environment variable.

Example:

```bash
docker run -e 'IAM_POLICY={"Version":"2012-10-17","Statement":{"Effect":"Allow","Resource":"*","Action":"ec2:*"}}' ...
```
