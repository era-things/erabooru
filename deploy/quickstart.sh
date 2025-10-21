#!/usr/bin/env bash
#
#  erabooru quick-start
#
#  curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash
#
set -euo pipefail

EMBED_VARIANT_OVERRIDE=${ERABOORU_EMBED_VARIANT:-}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --gpu)
            EMBED_VARIANT_OVERRIDE="gpu-cuda12"
            shift
            ;;
        --cpu)
            EMBED_VARIANT_OVERRIDE="cpu"
            shift
            ;;
        --embed-worker-variant=*)
            EMBED_VARIANT_OVERRIDE="${1#*=}"
            shift
            ;;
        -h|--help)
            cat <<'USAGE'
Usage: quickstart.sh [--cpu|--gpu|--embed-worker-variant=VARIANT]

Options:
  --cpu                     Force the CPU embed worker image.
  --gpu                     Force the CUDA embed worker image.
  --embed-worker-variant    Explicitly set the embed worker variant (cpu or gpu-cuda12).
  -h, --help                Show this message.

You can also set ERABOORU_EMBED_VARIANT in the environment.
USAGE
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            exit 1
            ;;
    esac
done

if [[ -n "$EMBED_VARIANT_OVERRIDE" ]]; then
    EMBED_VARIANT_OVERRIDE=$(printf '%s' "$EMBED_VARIANT_OVERRIDE" | tr '[:upper:]' '[:lower:]')
fi

DEST=${BOORU_HOME:-"$HOME/erabooru"}

echo "erabooru quick start"
echo "→ Using directory: $DEST"

# Check dependencies
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed. Please install Docker first:"
    echo "   https://docs.docker.com/get-docker/"
    exit 1
fi

if ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not available"
    exit 1
fi

mkdir -p "$DEST"
cd "$DEST"

# Helper function for downloads
download_file() {
    local url="$1"
    local output="$2"
    
    # Remove existing file/directory if it exists
    if [[ -e "$output" ]]; then
        rm -rf "$output"
    fi
    
    echo "→ Downloading $output..."
    
    # Use different approach on Windows/Git Bash
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
        # Use Windows curl explicitly and force file creation
        if curl.exe -fsSL "$url" --output "$output" 2>/dev/null; then
            echo "✅ Downloaded $output"
        elif powershell.exe -Command "Invoke-WebRequest -Uri '$url' -OutFile '$output'" 2>/dev/null; then
            echo "✅ Downloaded $output (via PowerShell)"
        else
            echo "❌ Failed to download $url"
            exit 1
        fi
    else
        # Unix/Linux/macOS
        if curl -fsSL "$url" -o "$output"; then
            echo "✅ Downloaded $output"
        else
            echo "❌ Failed to download $url"
            exit 1
        fi
    fi
    
    # Verify the file was created properly
    if [[ ! -f "$output" ]] || [[ ! -s "$output" ]]; then
        echo "❌ Downloaded file is invalid or empty"
        exit 1
    fi
}

validate_variant() {
    case "$1" in
        cpu|gpu-cuda12)
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

set_env_value() {
    local key="$1"
    local value="$2"
    local tmp

    if [[ -f .env ]] && grep -q "^${key}=" .env; then
        tmp=$(mktemp 2>/dev/null || echo ".env.tmp.$$")
        awk -v key="$key" -v value="$value" '
            BEGIN { replaced = 0 }
            $0 ~ "^" key "=" { print key "=" value; replaced = 1; next }
            { print }
            END { if (!replaced) print key "=" value }
        ' .env > "$tmp"
        mv "$tmp" .env
    else
        echo "${key}=${value}" >> .env
    fi
}

has_nvidia_gpu() {
    if command -v nvidia-smi >/dev/null 2>&1 && nvidia-smi -L >/dev/null 2>&1; then
        return 0
    fi
    if command -v nvidia-smi.exe >/dev/null 2>&1 && nvidia-smi.exe -L >/dev/null 2>&1; then
        return 0
    fi
    if docker info --format '{{json .Runtimes}}' 2>/dev/null | grep -q 'nvidia'; then
        return 0
    fi
    if docker info --format '{{json .Plugins.Runtime}}' 2>/dev/null | grep -q 'nvidia'; then
        return 0
    fi
    return 1
}

select_embed_variant() {
    local existing

    if [[ -n "$EMBED_VARIANT_OVERRIDE" ]]; then
        if ! validate_variant "$EMBED_VARIANT_OVERRIDE"; then
            echo "❌ Unsupported embed worker variant '$EMBED_VARIANT_OVERRIDE'. Allowed values: cpu, gpu-cuda12" >&2
            exit 1
        fi
        echo "→ Using embed worker variant from override: $EMBED_VARIANT_OVERRIDE"
        echo "$EMBED_VARIANT_OVERRIDE"
        return
    fi

    if [[ -f .env ]]; then
        existing=$(grep '^EMBED_WORKER_VARIANT=' .env 2>/dev/null | tail -n1 | cut -d= -f2- || true)
        if [[ -n "$existing" ]]; then
            if validate_variant "$existing"; then
                echo "→ Keeping existing EMBED_WORKER_VARIANT=$existing from .env"
                echo "$existing"
                return
            else
                echo "→ Existing EMBED_WORKER_VARIANT=$existing not recognized; falling back to auto-detection"
            fi
        fi
    fi

    if has_nvidia_gpu; then
        echo "→ NVIDIA GPU support detected; using gpu-cuda12 embed worker"
        echo "gpu-cuda12"
    else
        echo "→ No NVIDIA GPU detected; using cpu embed worker"
        echo "cpu"
    fi
}

# ────────────────────────────────────────────────────────────────
# 1. Create .env on first run
# ────────────────────────────────────────────────────────────────
if [[ ! -f .env ]]; then
    echo "→ Creating .env file..."
    download_file "https://raw.githubusercontent.com/era-things/erabooru/main/.env.example" ".env"
    
    # Ensure Unix line endings (important for Windows)
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
        sed -i 's/\r$//' .env
    fi
    
    echo "✅ Created .env file"
else
    echo "→ Using existing .env file."
fi

echo "→ Verifying .env file contents..."
echo "POSTGRES_HOST from .env: $(grep POSTGRES_HOST .env || echo 'NOT FOUND')"
echo "MINIO_ROOT_USER from .env: $(grep MINIO_ROOT_USER .env || echo 'NOT FOUND')"

variant=$(select_embed_variant)
set_env_value "EMBED_WORKER_VARIANT" "$variant"
echo "EMBED_WORKER_VARIANT resolved to: $variant"


# ────────────────────────────────────────────────────────────────
# 2. Download compose files
# ────────────────────────────────────────────────────────────────
echo "→ Downloading compose files..."
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/docker-compose.yml" "docker-compose.yml"
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/docker-compose.pull.yml" "docker-compose.pull.yml"
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/Caddyfile.prod" "Caddyfile.prod"

# ────────────────────────────────────────────────────────────────
# 3. Start services
# ────────────────────────────────────────────────────────────────
echo "→ Pulling container images..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml pull

echo "→ Starting erabooru..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml up -d

# ────────────────────────────────────────────────────────────────
# 4. Show status
# ────────────────────────────────────────────────────────────────
# Cross-platform IP detection
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" || "$OSTYPE" == "win32" ]]; then
    IP="localhost"
else
    IP=$(hostname -I | awk '{print $1}' 2>/dev/null || echo "localhost")
fi

if docker compose -f docker-compose.yml -f docker-compose.pull.yml ps | grep -q "Exit"; then
    echo "❌ Some services failed to start. Check logs with:"
    echo "   docker compose logs"
else
    cat <<EOF
    

🟢 erabooru is running!

• Main app       → http://$IP
• Logs          → docker compose logs
• Stop          → docker compose down

Update later:
  cd $DEST && curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash

EOF
fi