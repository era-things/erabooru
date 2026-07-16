#!/usr/bin/env bash
#
#  erabooru quick-start
#
#  curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash
#
set -euo pipefail

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
