#!/usr/bin/env bash
#
#  EraBooru quick-start
#
#  curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash
#
set -euo pipefail

DEST=${BOORU_HOME:-"$HOME/erabooru"}

echo "🚀 EraBooru Quick Start"
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
    echo "✅ Created .env file"
else
    echo "→ using existing .env file."
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
sudo chown -R 65532:65532 bleve-index || {
    echo "⚠️  Could not set ownership with sudo, trying with current user..."
    # Fallback: just ensure directory exists
    chmod 755 bleve-index
}

echo "→ Pulling container images..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml pull

echo "→ Starting EraBooru..."
docker compose -f docker-compose.yml -f docker-compose.pull.yml up -d

echo "→ Waiting for services..."
sleep 10

# ────────────────────────────────────────────────────────────────
# 4. Show status
# ────────────────────────────────────────────────────────────────
IP=$(hostname -I | awk '{print $1}' || echo "localhost")

if docker compose -f docker-compose.yml -f docker-compose.pull.yml ps | grep -q "Exit"; then
    echo "❌ Some services failed to start. Check logs with:"
    echo "   docker compose logs"
else
    cat <<EOF
    

🟢 EraBooru is running!

• Main app       → http://$IP
• MinIO console  → http://$IP/minio
• Logs          → docker compose logs
• Stop          → docker compose down

Update later:
  cd $DEST && curl -fsSL https://raw.githubusercontent.com/era-things/erabooru/main/deploy/quickstart.sh | bash

EOF
fi

echo "→ Checking container environment..."
sleep 5
echo "Container POSTGRES_HOST: $(docker compose -f docker-compose.yml -f docker-compose.pull.yml exec app printenv POSTGRES_HOST 2>/dev/null || echo 'NOT SET')"