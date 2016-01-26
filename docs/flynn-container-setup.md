By default, a job receives the default permissions configured by the metadata
proxy. However, some jobs require more or less permissions. For these cases, the
job can provide a custom IAM role, a custom IAM policy, or both.

# Job Role

A job can specify a specific role to use by setting the `IAM_ROLE` metadata
variable. The metadata proxy will return credentials for the given role when
requested.

Example:

```bash
flynn meta set 'IAM_ROLE=arn:aws:iam::123456789012:role/JobRoleName'
```

Note that the host machine’s instance profile must have permission to assume the given role.
If not, the job will receive an error when requesting the credentials.

# Job Policy

A job can specify a custom IAM policy by setting the `IAM_POLICY` metadata
variable. The resulting job permissions will be
the intersection of the custom policy and the default container role or the role
specified by the job's `IAM_ROLE` metdata variable.

Example:

```bash
flynn meta set 'IAM_POLICY={"Version":"2012-10-17","Statement":{"Effect":"Allow","Resource":"*","Action":"ec2:*"}}'
```
