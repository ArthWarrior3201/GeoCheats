Name: geocheats
Version: 1.1.0
Release: 1%{?dist}
Summary: Local Geometry Dash mod controller and Dash authoring tool
License: MIT AND BSD-2-Clause AND BSD-3-Clause
BuildArch: x86_64
Source0: geocheats-files.tar.gz
Requires: glibc >= 2.34
Requires: xdg-utils
AutoReqProv: no
%global debug_package %{nil}
%description
GeoCheats opens local controls in your browser. Linux gameplay targets the
Windows game under Proton/Wine and existing OS process-access permissions.
%prep
%build
%install
mkdir -p %{buildroot}
tar -xzf %{SOURCE0} -C %{buildroot}
%files
/usr/bin/geocheats
/usr/bin/geocheats-dash
/usr/share/applications/geocheats.desktop
%doc /usr/share/doc/geocheats/mod_building_guide.txt
