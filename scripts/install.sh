#!/usr/bin/env bash
set -euo pipefail

REPO="Hezekiah-Elisha/tazx"
BINARY_NAME="tazx"
VERSION="latest"
INSTALL_DIR=""
FROM_SOURCE=false
UPDATE_ONLY=false
UNINSTALL=false
TMPDIR_PATH=""

SCRIPT_PATH="${BASH_SOURCE[0]}"
SCRIPT_DIR="$(cd "$(dirname "${SCRIPT_PATH}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

cleanup() {
    if [[ -n "${TMPDIR_PATH}" && -d "${TMPDIR_PATH}" ]]; then
        rm -rf "${TMPDIR_PATH}"
    fi
}
trap cleanup EXIT INT TERM

usage() {
    cat <<'EOF'
Tazx Installer

Usage:
  install.sh [options]

Options:
  --version <tag>      Install a specific release tag (e.g. v1.1.0). Default: latest
  --install-dir <dir>  Target installation directory.
                       Default: active writable tazx dir, /usr/local/bin (root), or $HOME/.local/bin
  --from-source        Build and install the latest version from the local Go repository
  --update             Update an existing Tazx installation in-place
  --uninstall          Remove the installed tazx binary from --install-dir
  -h, --help           Show this help message
EOF
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --version)
            [[ $# -lt 2 ]] && { echo "Error: --version requires an argument" >&2; exit 1; }
            VERSION="$2"
            shift 2
            ;;
        --install-dir)
            [[ $# -lt 2 ]] && { echo "Error: --install-dir requires an argument" >&2; exit 1; }
            INSTALL_DIR="$2"
            shift 2
            ;;
        --from-source)
            FROM_SOURCE=true
            shift
            ;;
        --update)
            UPDATE_ONLY=true
            shift
            ;;
        --uninstall)
            UNINSTALL=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Error: Unknown option '$1'" >&2
            usage >&2
            exit 1
            ;;
    esac
done

find_existing_binary() {
    if command -v "${BINARY_NAME}" >/dev/null 2>&1; then
        command -v "${BINARY_NAME}"
        return 0
    fi
    if [[ -x "${HOME}/.local/bin/${BINARY_NAME}" ]]; then
        echo "${HOME}/.local/bin/${BINARY_NAME}"
        return 0
    fi
    if [[ -x "/usr/local/bin/${BINARY_NAME}" ]]; then
        echo "/usr/local/bin/${BINARY_NAME}"
        return 0
    fi
    return 1
}

if [[ -z "${INSTALL_DIR}" ]]; then
    EXISTING_FOUND="$(find_existing_binary || true)"
    if [[ "${UPDATE_ONLY}" == true ]]; then
        if [[ -z "${EXISTING_FOUND}" ]]; then
            echo "Error: No existing Tazx installation found to update." >&2
            echo "Run without --update to perform a fresh installation." >&2
            exit 1
        fi
        INSTALL_DIR="$(dirname "${EXISTING_FOUND}")"
    elif [[ -n "${EXISTING_FOUND}" && -w "$(dirname "${EXISTING_FOUND}")" ]]; then
        INSTALL_DIR="$(dirname "${EXISTING_FOUND}")"
    elif [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="${HOME}/.local/bin"
    fi
fi

TARGET_BIN="${INSTALL_DIR}/${BINARY_NAME}"

if [[ "${UNINSTALL}" == true ]]; then
    if [[ -e "${TARGET_BIN}" ]]; then
        if [[ ! -w "${INSTALL_DIR}" ]]; then
            echo "Error: No write permission for ${INSTALL_DIR}." >&2
            echo "Run with sufficient permissions or specify --install-dir." >&2
            exit 1
        fi
        rm -f "${TARGET_BIN}"
        echo "Removed ${TARGET_BIN}."
        echo "Note: Any optional configuration file at ${HOME}/.tazx.yaml was preserved."
    else
        echo "Tazx binary not found at ${TARGET_BIN}."
    fi
    exit 0
fi

detect_platform() {
    local os arch
    os="$(uname -s)"
    arch="$(uname -m)"

    case "${os}" in
        Linux) OS="linux" ;;
        Darwin) OS="darwin" ;;
        *)
            echo "Error: Unsupported operating system '${os}'. Supported: Linux, macOS." >&2
            exit 1
            ;;
    esac

    case "${arch}" in
        x86_64|amd64) ARCH="amd64" ;;
        aarch64|arm64) ARCH="arm64" ;;
        *)
            echo "Error: Unsupported CPU architecture '${arch}'. Supported: amd64, arm64." >&2
            exit 1
            ;;
    esac
}

verify_sha256() {
    local file_path="$1"
    local checksums_path="$2"
    local asset_name="$3"

    local expected
    expected="$(awk -v name="${asset_name}" '$2 == name || $2 == ("*" name) { print $1; exit }' "${checksums_path}")"
    if [[ -z "${expected}" ]]; then
        echo "Error: Checksum entry for '${asset_name}' not found in checksums.txt." >&2
        exit 1
    fi

    local actual=""
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "${file_path}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "${file_path}" | awk '{print $1}')"
    else
        echo "Error: Neither 'sha256sum' nor 'shasum' is available to verify binary integrity." >&2
        exit 1
    fi

    if [[ "${expected}" != "${actual}" ]]; then
        echo "Error: SHA-256 verification failed for ${asset_name}!" >&2
        echo "  Expected: ${expected}" >&2
        echo "  Actual:   ${actual}" >&2
        exit 1
    fi
    echo "Checksum verified (${actual})."
}

ensure_install_dir() {
    if [[ ! -d "${INSTALL_DIR}" ]]; then
        mkdir -p "${INSTALL_DIR}" 2>/dev/null || {
            echo "Error: Unable to create directory ${INSTALL_DIR}." >&2
            echo "Choose a user-writable directory (such as ${HOME}/.local/bin) or rerun with sudo if installing system-wide." >&2
            exit 1
        }
    fi
    if [[ ! -w "${INSTALL_DIR}" ]]; then
        echo "Error: ${INSTALL_DIR} is not writable by current user '$(id -un)'." >&2
        if [[ "${FROM_SOURCE}" == true ]]; then
            echo "To update ${TARGET_BIN}, run: sudo bash \"${SCRIPT_PATH}\" --from-source --install-dir \"${INSTALL_DIR}\"" >&2
        else
            echo "To update ${TARGET_BIN}, use 'sudo' or install to ${HOME}/.local/bin instead." >&2
        fi
        exit 1
    fi
}

check_existing() {
    if [[ -x "${TARGET_BIN}" ]]; then
        echo "Existing installation detected at ${TARGET_BIN}; upgrading in-place..."
    fi
}

TMPDIR_PATH="$(mktemp -d)"
ensure_install_dir
check_existing

if [[ "${FROM_SOURCE}" == true ]]; then
    if ! command -v go >/dev/null 2>&1; then
        echo "Error: Go toolchain not found. Install Go or omit --from-source to download a release binary." >&2
        exit 1
    fi

    SRC_DIR=""
    if [[ -f "${REPO_ROOT}/go.mod" && -f "${REPO_ROOT}/main.go" ]]; then
        SRC_DIR="${REPO_ROOT}"
    elif [[ -f "./go.mod" && -f "./main.go" ]]; then
        SRC_DIR="$(pwd)"
    else
        echo "Error: --from-source could not locate the Tazx Go repository (go.mod/main.go)." >&2
        exit 1
    fi

    LDFLAGS="-s -w"
    if [[ "${VERSION}" != "latest" ]]; then
        LDFLAGS="${LDFLAGS} -X github.com/Hezekiah-Elisha/tazx/cmd.AppVersion=${VERSION}"
    fi

    echo "Building latest Tazx from source (${SRC_DIR})..."
    (cd "${SRC_DIR}" && go build -trimpath -ldflags "${LDFLAGS}" -o "${TMPDIR_PATH}/${BINARY_NAME}" .)
    "${TMPDIR_PATH}/${BINARY_NAME}" version --no-color >/dev/null
else
    if ! command -v curl >/dev/null 2>&1; then
        echo "Error: 'curl' is required to download release artifacts." >&2
        exit 1
    fi

    detect_platform
    ASSET_NAME="${BINARY_NAME}-${OS}-${ARCH}"
    if [[ "${VERSION}" == "latest" ]]; then
        BASE_URL="https://github.com/${REPO}/releases/latest/download"
    else
        BASE_URL="https://github.com/${REPO}/releases/download/${VERSION}"
    fi

    echo "Downloading ${ASSET_NAME} from ${BASE_URL}..."
    curl -fsSL --proto '=https' --tlsv1.2 "${BASE_URL}/${ASSET_NAME}" -o "${TMPDIR_PATH}/${BINARY_NAME}"
    curl -fsSL --proto '=https' --tlsv1.2 "${BASE_URL}/checksums.txt" -o "${TMPDIR_PATH}/checksums.txt"

    verify_sha256 "${TMPDIR_PATH}/${BINARY_NAME}" "${TMPDIR_PATH}/checksums.txt" "${ASSET_NAME}"
fi

install -m 0755 "${TMPDIR_PATH}/${BINARY_NAME}" "${TARGET_BIN}"
INSTALLED_VER="$("${TARGET_BIN}" version --no-color | awk -F': *' '/^Version:/ {v=$2} END {print v}')"

echo ""
echo "TAZX INSTALLED SUCCESSFULLY"
echo "Installed binary: ${TARGET_BIN} (${INSTALLED_VER})"
echo ""

ACTIVE_BIN="$(command -v "${BINARY_NAME}" || true)"
if [[ -n "${ACTIVE_BIN}" && "${ACTIVE_BIN}" != "${TARGET_BIN}" ]]; then
    SUDO_PREFIX=""
    if [[ ! -w "$(dirname "${ACTIVE_BIN}")" ]]; then
        SUDO_PREFIX="sudo "
    fi
    echo "Warning: Another '${BINARY_NAME}' earlier in PATH takes precedence: ${ACTIVE_BIN}" >&2
    echo "Running bare '${BINARY_NAME}' will execute ${ACTIVE_BIN} instead of ${TARGET_BIN}." >&2
    echo "To update ${ACTIVE_BIN}, run:" >&2
    if [[ "${FROM_SOURCE}" == true ]]; then
        echo "  ${SUDO_PREFIX}bash \"${SCRIPT_PATH}\" --from-source --install-dir \"$(dirname "${ACTIVE_BIN}")\"" >&2
    else
        echo "  ${SUDO_PREFIX}bash \"${SCRIPT_PATH}\" --install-dir \"$(dirname "${ACTIVE_BIN}")\"" >&2
    fi
    echo "Or remove the older copy: ${SUDO_PREFIX}rm -f \"${ACTIVE_BIN}\"" >&2
    echo "" >&2
fi

case ":${PATH}:" in
    *":${INSTALL_DIR}:"*)
        ;;
    *)
        echo "Note: ${INSTALL_DIR} is not currently in your PATH."
        echo "Add it to your shell profile (~/.bashrc or ~/.zshrc) by adding:"
        echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
        echo ""
        ;;
esac

echo "Run:"
echo "  tazx version"
echo "  tazx status"
echo "  tazx doctor"
