Name: geocheats
Version: 1.3.1
Release: 1%{?dist}
Summary: Local Geometry Dash mod controller and Dash authoring tool
License: MIT AND BSD-2-Clause AND BSD-3-Clause
BuildArch: x86_64
Source0: geocheats-files.tar.gz
Requires: glibc >= 2.34
Requires: xdg-utils
Requires: libGL.so.1()(64bit)
Requires: libX11.so.6()(64bit)
Requires: libXcursor.so.1()(64bit)
Requires: libXrandr.so.2()(64bit)
Requires: libXinerama.so.1()(64bit)
Requires: libXi.so.6()(64bit)
Requires: libXxf86vm.so.1()(64bit)
Requires: zenity
AutoReqProv: no
%global debug_package %{nil}
%description
GeoCheats provides native desktop controls. Linux gameplay targets the
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
/usr/share/applications/dash.desktop
/usr/share/icons/hicolor/256x256/apps/geocheats.png
/usr/share/icons/hicolor/256x256/apps/geocheats-dash.png
/usr/share/icons/hicolor/512x512/apps/geocheats.png
/usr/share/icons/hicolor/512x512/apps/geocheats-dash.png
%doc /usr/share/doc/geocheats/mod_building_guide.txt
%doc /usr/share/doc/geocheats/THIRD_PARTY_NOTICES.txt
