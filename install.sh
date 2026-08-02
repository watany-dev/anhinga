#!/bin/sh

set -eu

REPOSITORY="watany-dev/anhinga"
INSTALL_DIR=${ANHINGA_INSTALL_DIR:-/usr/local/bin}

die() {
    printf 'Error: %s\n' "$*" >&2
    exit 1
}

require_command() {
    command -v "$1" >/dev/null 2>&1 || die "$1 is not installed."
}

normalize_version() {
    version=${1#v}
    case "$version" in
        ''|*[!0-9A-Za-z.+-]*) return 1 ;;
    esac
    printf '%s\n' "$version"
}

detect_arch() {
    case $(uname -m) in
        x86_64|amd64) printf '%s\n' x86_64 ;;
        arm64|aarch64) printf '%s\n' arm64 ;;
        *) return 1 ;;
    esac
}

detect_os() {
    case $(uname -s) in
        Linux) printf '%s\n' Linux ;;
        Darwin) printf '%s\n' Darwin ;;
        MINGW*|MSYS*|CYGWIN*) printf '%s\n' Windows ;;
        *) return 1 ;;
    esac
}

fetch_latest_version() {
    curl --proto '=https' --tlsv1.2 -fsSL \
        -H 'Accept: application/vnd.github+json' \
        -H 'X-GitHub-Api-Version: 2022-11-28' \
        "https://api.github.com/repos/${REPOSITORY}/releases/latest" |
        sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"v\{0,1\}\([^"]*\)".*/\1/p' |
        sed -n '1p'
}

verify_checksum() {
    checksums=$1
    archive_name=$2
    archive=$3
    expected=$(awk -v name="$archive_name" '$2 == name { print $1; exit }' "$checksums")
    [ -n "$expected" ] || return 1

    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$archive" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$archive" | awk '{print $1}')
    else
        return 1
    fi
    [ "$actual" = "$expected" ]
}

install_binary() {
    source_file=$1
    destination=$2

    if [ -w "$INSTALL_DIR" ]; then
        install -m 0755 "$source_file" "$destination"
    else
        require_command sudo
        sudo install -m 0755 "$source_file" "$destination"
    fi
}

main() {
    require_command curl
    require_command install

    if [ "$#" -gt 1 ]; then
        die "usage: install.sh [version]"
    fi

    if [ "$#" -eq 1 ]; then
        requested_version=$1
    else
        requested_version=$(fetch_latest_version) || die "failed to fetch the latest version."
    fi
    version=$(normalize_version "$requested_version") || die "invalid version: $requested_version"

    arch=$(detect_arch) || die "unsupported architecture: $(uname -m)"
    os=$(detect_os) || die "unsupported operating system: $(uname -s)"
    if [ "$os" = Windows ]; then
        extension=zip
        binary_name=anhinga.exe
        require_command unzip
    else
        extension=tar.gz
        binary_name=anhinga
        require_command tar
    fi

    archive_name="anhinga_${os}_${arch}.${extension}"
    release_url="https://github.com/${REPOSITORY}/releases/download/v${version}"
    temporary_directory=$(mktemp -d) || die "could not create a temporary directory."
    trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM

    archive="${temporary_directory}/${archive_name}"
    checksums="${temporary_directory}/checksums.txt"
    printf 'Downloading anhinga v%s...\n' "$version"
    curl --proto '=https' --tlsv1.2 -fsSL -o "$archive" "${release_url}/${archive_name}" ||
        die "failed to download ${archive_name}."
    curl --proto '=https' --tlsv1.2 -fsSL -o "$checksums" "${release_url}/checksums.txt" ||
        die "failed to download checksums.txt."
    verify_checksum "$checksums" "$archive_name" "$archive" ||
        die "checksum verification failed for ${archive_name}."

    extract_directory="${temporary_directory}/extract"
    mkdir "$extract_directory"
    if [ "$extension" = tar.gz ]; then
        tar -xzf "$archive" -C "$extract_directory" "$binary_name" ||
            die "failed to extract ${binary_name}."
    else
        unzip -q "$archive" "$binary_name" -d "$extract_directory" ||
            die "failed to extract ${binary_name}."
    fi

    [ -f "${extract_directory}/${binary_name}" ] && [ ! -L "${extract_directory}/${binary_name}" ] ||
        die "release did not contain a regular ${binary_name} binary."

    install_binary "${extract_directory}/${binary_name}" "${INSTALL_DIR}/${binary_name}"
    printf 'Installed anhinga v%s to %s/%s\n' "$version" "$INSTALL_DIR" "$binary_name"
}

if [ "${ANHINGA_INSTALL_TESTING:-0}" != 1 ]; then
    main "$@"
fi
