#!/bin/bash

set -e

echo "🧰 Installation du proxy SOCKS5 avec rotation IPv6..."

# Variables
GO_VERSION="1.24.2"
GO_TARBALL="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_TARBALL}"
INSTALL_DIR="/usr/local"
APP_DIR="/opt/socks5proxy"
REPO_URL="https://github.com/lekr74/go-rotating-proxy.git"
BINARY_NAME="socks5proxy"
SERVICE_NAME="socks5proxy.service"

# 1. Installer Go
echo "⬇️ Téléchargement de Go ${GO_VERSION}..."
cd /tmp
curl -LO "${GO_URL}"
sudo rm -rf ${INSTALL_DIR}/go
sudo tar -C ${INSTALL_DIR} -xzf "${GO_TARBALL}"
echo "✅ Go ${GO_VERSION} installé."

# 2. Exporter le PATH
export PATH=$PATH:/usr/local/go/bin
if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
    echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
fi

# 3. Cloner le repo
echo "📦 Clonage du projet SOCKS5..."
sudo rm -rf ${APP_DIR}
sudo git clone "${REPO_URL}" "${APP_DIR}"
cd "${APP_DIR}"

# 4. Installer les dépendances
echo "📚 Installation des dépendances..."
go mod tidy
go get github.com/fsnotify/fsnotify

# 5. Build
echo "🔨 Compilation du binaire..."
go build -o ${BINARY_NAME}

# 6. Installer le binaire
sudo cp ${BINARY_NAME} /usr/local/bin/

# 7. Installer le service systemd
echo "🛠 Installation du service systemd..."
cat <<EOF | sudo tee /etc/systemd/system/${SERVICE_NAME}
[Unit]
Description=SOCKS5 Proxy avec rotation IPv6
After=network.target

[Service]
ExecStart=/usr/local/bin/${BINARY_NAME}
WorkingDirectory=${APP_DIR}
Restart=always
User=root
Environment=PATH=/usr/local/go/bin:/usr/bin:/bin

[Install]
WantedBy=multi-user.target
EOF

# 8. Activer et démarrer le service
sudo systemctl daemon-reexec
sudo systemctl enable --now ${SERVICE_NAME}

# 9. Affichage final
echo ""
echo "✅ Installation terminée avec succès !"
echo "➡️ Le proxy tourne en tant que service : systemctl status ${SERVICE_NAME}"
echo "➡️ Fichiers de config attendus : ${APP_DIR}/users.yaml & subnets.json"
