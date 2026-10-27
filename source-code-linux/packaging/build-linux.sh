#!/usr/bin/env bash
set -euo pipefail
command -v dpkg-deb >/dev/null
command -v rpmbuild >/dev/null
build_root=$(mktemp -d)
mkdir -p "$build_root/deb/DEBIAN" "$build_root/deb/usr/bin" "$build_root/deb/usr/share/applications" "$build_root/deb/usr/share/doc/geocheats" "$build_root/rpm/SPECS" "$build_root/rpm/BUILD" "$build_root/rpm/RPMS" "$build_root/rpm/SRPMS" "$build_root/rpm/SOURCES" "$build_root/rpm/BUILDROOT" packages
install -m755 GeoCheats "$build_root/deb/usr/bin/geocheats"
install -m755 Dash "$build_root/deb/usr/bin/geocheats-dash"
install -m644 mod_building_guide.txt "$build_root/deb/usr/share/doc/geocheats/"
install -m644 packaging/geocheats.desktop "$build_root/deb/usr/share/applications/"
cp packaging/control "$build_root/deb/DEBIAN/control"
dpkg-deb --root-owner-group --build "$build_root/deb" packages/geocheats_1.1.0_amd64.deb
tar -C "$build_root/deb" --exclude=DEBIAN -czf "$build_root/rpm/SOURCES/geocheats-files.tar.gz" usr
rpmbuild --define "_topdir $build_root/rpm" -bb packaging/geocheats.spec
find "$build_root/rpm/RPMS" -name '*.rpm' -exec cp '{}' packages/ \;
cp README.md packages/README.md
cd packages
zip -j ../GeoCheats-Linux.zip ./*.deb ./*.rpm README.md
