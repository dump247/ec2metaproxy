Summary: Docker Container EC2 Metadata Proxy
Name: docker-ec2-metadata
Version: 1.0
Release: 1
License: MIT
Group: System Environment/Daemons
Vendor: github.com/dump247
Requires: docker-io >= 1.2, iptables

%description
Service that intercepts calls from docker containers to the EC2 metadata service.
The proxy can then alter results for specific containers, such as IAM credentials.

%prep

%build
pushd $GOPATH/rpm/service
cp -r . $RPM_BUILD_ROOT/
popd

mkdir -p $RPM_BUILD_ROOT/var/log/docker-ec2-metadata
mkdir -p $RPM_BUILD_ROOT/var/lib/docker-ec2-metadata

mkdir -p $RPM_BUILD_ROOT/usr/lib/docker-ec2-metadata/bin
cp $GOPATH/bin/docker-ec2-metadata $RPM_BUILD_ROOT/usr/lib/docker-ec2-metadata/bin/docker-ec2-metadata

%pre
getent passwd docker-ec2-metadata ||
    useradd                             \
        --user-group                    \
        --system                        \
        -G docker                       \
        -d /usr/lib/docker-ec2-metadata \
        -s /sbin/nologin                \
        docker-ec2-metadata

%post
chkconfig /etc/init.d/docker-ec2-metadata --add
chkconfig docker-ec2-metadata on

%preun
service docker-ec2-metadata stop-clean
chkconfig docker-ec2-metadata off
chkconfig /etc/init.d/docker-ec2-metadata --del

%postun
userdel docker-ec2-metadata

%files
# Logging
%dir /var/log/docker-ec2-metadata
/etc/logrotate.d/docker-ec2-metadata

# Service
/etc/sysconfig/docker-ec2-metadata
%attr(0755, root, root) /etc/init.d/docker-ec2-metadata

# Temp data
%dir %attr(0755, docker-ec2-metadata, docker-ec2-metadata) /var/lib/docker-ec2-metadata

# Application
%dir /usr/lib/docker-ec2-metadata
%dir /usr/lib/docker-ec2-metadata/bin
%attr(0755, root, root) /usr/lib/docker-ec2-metadata/bin/docker-ec2-metadata
%attr(0755, root, root) /usr/lib/docker-ec2-metadata/bin/add-iptables-rules
