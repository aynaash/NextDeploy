#!/bin/bash
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

check_sudo() {
  if sudo -n true 2>/dev/null; then
    echo -e "${GREEN}✓ Passwordless sudo configured${NC}"
  else
    echo -e "${RED}✗ Passwordless sudo required${NC}"
    echo "Run:"
    echo "  echo '$USER ALL=(ALL) NOPASSWD:ALL' | sudo tee /etc/sudoers.d/90-nextdeploy-user"
    echo "  sudo chmod 440 /etc/sudoers.d/90-nextdeploy-user"
    exit 1
  fi
}

check_docker() {
  if docker --version &>/dev/null; then
    echo -e "${GREEN}✓ Docker installed${NC}"
  else
    echo -e "${RED}✗ Docker not found${NC}"
    echo "Install with:"
    echo "  curl -fsSL https://get.docker.com | sudo sh"
    exit 1
  fi
}

check_sudo
check_docker

echo -e "\n${GREEN}Server meets NextDeploy requirements!${NC}"
