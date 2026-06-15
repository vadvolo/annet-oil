#!/bin/bash

echo "=== Docker Diagnostics for Linux ==="
echo

echo "1. Checking if Docker service is running..."
if systemctl is-active --quiet docker; then
    echo "✅ Docker service is running"
    systemctl status docker --no-pager -l
else
    echo "❌ Docker service is NOT running"
    echo "Try: sudo systemctl start docker"
fi

echo
echo "2. Checking Docker socket..."
if [ -e /var/run/docker.sock ]; then
    echo "✅ Docker socket exists"
    ls -la /var/run/docker.sock
else
    echo "❌ Docker socket does not exist"
fi

echo
echo "3. Checking current user permissions..."
if groups $USER | grep -q docker; then
    echo "✅ User $USER is in docker group"
else
    echo "❌ User $USER is NOT in docker group"
    echo "Try: sudo usermod -aG docker $USER && newgrp docker"
fi

echo
echo "4. Checking Docker daemon connectivity..."
if docker info > /dev/null 2>&1; then
    echo "✅ Can connect to Docker daemon"
    docker version
else
    echo "❌ Cannot connect to Docker daemon"
    echo "Try running with sudo: sudo ./bin/annet-oil containers list --format json"
fi

echo
echo "5. Environment variables..."
echo "DOCKER_HOST: ${DOCKER_HOST:-not set}"
echo "DOCKER_TLS_VERIFY: ${DOCKER_TLS_VERIFY:-not set}"
echo "DOCKER_CERT_PATH: ${DOCKER_CERT_PATH:-not set}"

echo
echo "=== Common fixes ==="
echo "1. Start Docker: sudo systemctl start docker"
echo "2. Enable Docker on boot: sudo systemctl enable docker"
echo "3. Add user to docker group: sudo usermod -aG docker \$USER"
echo "4. Refresh group membership: newgrp docker"
echo "5. Or run with sudo: sudo ./bin/annet-oil containers list --format json"