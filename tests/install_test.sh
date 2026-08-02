#!/bin/sh

set -eu

ANHINGA_INSTALL_TESTING=1
export ANHINGA_INSTALL_TESTING
. "$(dirname "$0")/../install.sh"

fail() {
    printf 'FAIL: %s\n' "$*" >&2
    exit 1
}

assert_equal() {
    [ "$1" = "$2" ] || fail "expected '$2', got '$1'"
}

assert_equal "$(normalize_version v1.2.3)" "1.2.3"
assert_equal "$(normalize_version 1.2.3-rc.1)" "1.2.3-rc.1"
if normalize_version '../../1.2.3' >/dev/null 2>&1; then
    fail "unsafe versions must be rejected"
fi

test_directory=$(mktemp -d)
trap 'rm -rf "$test_directory"' EXIT HUP INT TERM
archive_name=anhinga_Linux_x86_64.tar.gz
archive="${test_directory}/${archive_name}"
checksums="${test_directory}/checksums.txt"
printf 'release archive' >"$archive"
if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "$archive" | awk '{print $1}')
else
    digest=$(shasum -a 256 "$archive" | awk '{print $1}')
fi
printf '%s  %s\n' "$digest" "$archive_name" >"$checksums"

verify_checksum "$checksums" "$archive_name" "$archive" || fail "valid checksum was rejected"
printf 'tampered' >>"$archive"
if verify_checksum "$checksums" "$archive_name" "$archive"; then
    fail "tampered archive was accepted"
fi

# Exercise the complete installer without making a network request.
fixture_directory="${test_directory}/fixture"
fake_bin_directory="${test_directory}/fake-bin"
payload_directory="${test_directory}/payload"
install_directory="${test_directory}/install"
mkdir "$fixture_directory" "$fake_bin_directory" "$payload_directory" "$install_directory"
printf '%s\n' '#!/bin/sh' 'printf "fixture binary\\n"' >"${payload_directory}/anhinga"
chmod +x "${payload_directory}/anhinga"
tar -czf "${fixture_directory}/${archive_name}" -C "$payload_directory" anhinga
if command -v sha256sum >/dev/null 2>&1; then
    digest=$(sha256sum "${fixture_directory}/${archive_name}" | awk '{print $1}')
else
    digest=$(shasum -a 256 "${fixture_directory}/${archive_name}" | awk '{print $1}')
fi
printf '%s  %s\n' "$digest" "$archive_name" >"${fixture_directory}/checksums.txt"

printf '%s\n' \
    '#!/bin/sh' \
    'set -eu' \
    'output=' \
    'url=' \
    'while [ "$#" -gt 0 ]; do' \
    '    case "$1" in' \
    '        -o) output=$2; shift 2 ;;' \
    '        https://*) url=$1; shift ;;' \
    '        *) shift ;;' \
    '    esac' \
    'done' \
    'cp "${ANHINGA_TEST_FIXTURE}/${url##*/}" "$output"' \
    >"${fake_bin_directory}/curl"
chmod +x "${fake_bin_directory}/curl"

PATH="${fake_bin_directory}:$PATH" \
ANHINGA_INSTALL_DIR="$install_directory" \
ANHINGA_INSTALL_TESTING=0 \
ANHINGA_TEST_FIXTURE="$fixture_directory" \
    sh "$(dirname "$0")/../install.sh" 1.2.3 >/dev/null

[ -x "${install_directory}/anhinga" ] || fail "installer did not create an executable"
assert_equal "$("${install_directory}/anhinga")" "fixture binary"

printf 'install tests passed\n'
