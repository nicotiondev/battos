#!/bin/sh
set -e

REPO="nicotiondev/battos"
BINARY="battos"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detectar OS y arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Arch no soportada: $ARCH"; exit 1 ;;
esac

# Obtener última versión
VERSION=$(curl -sL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
  echo "No se pudo obtener la versión"; exit 1
fi

# Descargar
FILENAME="${BINARY}_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"
echo "Instalando $BINARY $VERSION..."
curl -sL "$URL" | tar -xz -C /tmp "$BINARY"
mkdir -p "$INSTALL_DIR"
mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
chmod +x "$INSTALL_DIR/$BINARY"

echo "✓ $BINARY instalado en $INSTALL_DIR/$BINARY"
echo "  Agregar $INSTALL_DIR al PATH si no está."
