Name:           svc-registry
Version:        %{registry_version}
Release:        1
Summary:        Registry of services: projects, branches, deployments, links and docs
License:        Apache-2.0
URL:            https://github.com/hoknie/service-registry
AutoReqProv:    no
Requires:       ca-certificates

%global debug_package %{nil}
%global __os_install_post %{nil}

%description
svc-registry keeps projects in nested organizations and folders, imports repositories from
GitHub, GitLab and Forgejo, records deployments from CI and Kubernetes, builds observability
links and collects documentation for an MCP server. One static binary and its interface in
/usr/share/svc-registry/web, run by systemd as the user svc-registry against an external Postgres.

%install
cp -a %{_sourcedir}/. %{buildroot}/

%files
/usr/bin/svc-registry
/usr/lib/svc-registry
/usr/lib/systemd/system/svc-registry.service
/usr/lib/sysusers.d/svc-registry.conf
/usr/lib/tmpfiles.d/svc-registry.conf
/usr/share/svc-registry
%doc /usr/share/doc/svc-registry

%post
if [ "$1" -eq 1 ]; then
    /usr/lib/svc-registry/after-install first
else
    /usr/lib/svc-registry/after-install upgrade
fi

%preun
if [ "$1" -eq 0 ]; then
    /usr/lib/svc-registry/before-remove
fi

%postun
if [ -d /run/systemd/system ]; then
    systemctl daemon-reload || true
fi
