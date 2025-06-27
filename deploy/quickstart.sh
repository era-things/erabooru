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
    local retries=3
    
    for i in $(seq 1 $retries); do
        if curl -fsSL "$url" -o "$output"; then
            return 0
        fi
        echo "⚠️  Download failed (attempt $i/$retries), retrying..."
        sleep 2
    done
    
    echo "❌ Failed to download $url"
    exit 1
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


# ────────────────────────────────────────────────────────────────
# 2. Download compose files
# ────────────────────────────────────────────────────────────────
echo "→ Downloading compose files..."
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/docker-compose.yml" "docker-compose.yml"
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/docker-compose.pull.yml" "docker-compose.pull.yml"
download_file "https://raw.githubusercontent.com/era-things/erabooru/main/Caddyfile" "Caddyfile"

# ────────────────────────────────────────────────────────────────
# 3. Start services
# ────────────────────────────────────────────────────────────────
# Create bleve index directory with correct permissions
echo "→ Setting up Bleve index directory..."
mkdir -p bleve-index
if command -v sudo >/dev/null 2>&1 && sudo -n true 2>/dev/null; then
    # sudo is available and works
    echo "→ Setting ownership with sudo..."
    sudo chown -R 65532:65532 bleve-index
elif [[ $(id -u) -eq 0 ]]; then
    # Running as root
    echo "→ Setting ownership as root..."
    chown -R 65532:65532 bleve-index
else
    # No sudo available or not root - use alternative approach
    echo "⚠️  No sudo available. Using alternative permissions approach..."
    
    # Make directory world-writable as fallback
    chmod 777 bleve-index
    
    # Try to create the index with current user first, then fix ownership in container
    echo "→ Will fix permissions after container starts..."
    NEED_PERMISSION_FIX=true
fi

echo "→ Pulling container images..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml pull

echo "→ Starting erabooru..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml up -d

# Wait longer and check if containers are actually ready
echo "→ Waiting for services to initialize..."
sleep 15  # Increased from 10

# Fix permissions if needed
if [[ "${NEED_PERMISSION_FIX:-false}" == "true" ]]; then
    echo "→ Fixing Bleve index permissions in container..."
    
    # Stop app temporarily
    docker compose -f docker-compose.yml -f docker-compose.pull.yml stop app
    
    # Run a temporary container to fix permissions
    docker compose -f docker-compose.yml -f docker-compose.pull.yml run --rm --user root app sh -c "
        chown -R 65532:65532 /data/bleve
        chmod -R 755 /data/bleve
    " || echo "⚠️  Could not fix permissions in container"
    
    # Start app again
    docker compose -f docker-compose.yml -f docker-compose.pull.yml start app
    sleep 5
fi

# Wait for app to respond to HTTP requests
echo "→ Waiting for app to be ready..."
for i in {1..15}; do
    if curl -s http://localhost/ >/dev/null 2>&1; then
        echo "✅ App is ready"
        break
    fi
    if [ $i -eq 15 ]; then
        echo "⚠️  App taking longer than expected. Restarting..."
        docker compose -f docker-compose.yml -f docker-compose.pull.yml restart app
        sleep 5
        break
    fi
    sleep 2
done

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
• MinIO console  → http://$IP/minio
• Logs          → docker compose logs
• Stop          → docker compose down

Update later:
  cd $DEST && curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash

EOF
fi