#!/bin/sh
# install.sh — instalador de BattOS
#
# Uso:
#   curl -fsSL https://raw.githubusercontent.com/nicotiondev/battos/master/scripts/install.sh | sh
#
# Variables opcionales:
#   INSTALL_DIR   directorio donde instalar los binarios (default: ~/.local/bin)
#   VERSION       versión específica a instalar (default: la última release)

set -e

REPO="nicotiondev/battos"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

# Detectar OS y arch
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case $ARCH in
  x86_64)       ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) echo "Arquitectura no soportada: $ARCH"; exit 1 ;;
esac

# Obtener última versión si no se especificó
if [ -z "$VERSION" ]; then
  VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | cut -d'"' -f4)
fi
if [ -z "$VERSION" ]; then
  echo "Error: no se pudo obtener la versión. Especificá VERSION=vX.Y.Z o revisá que el repo tenga releases."
  exit 1
fi

# Descargar y extraer el archive (contiene battos + battos-api + config)
FILENAME="battos_${VERSION}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/$REPO/releases/download/$VERSION/$FILENAME"

echo "Instalando BattOS $VERSION para ${OS}/${ARCH}..."
mkdir -p "$INSTALL_DIR"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

curl -fsSL "$URL" | tar -xz -C "$TMP"

# Instalar ambos binarios (battos y battos-api deben estar juntos para que `battos serve` funcione)
for BIN in battos battos-api; do
  if [ -f "$TMP/$BIN" ]; then
    mv "$TMP/$BIN" "$INSTALL_DIR/$BIN"
    chmod +x "$INSTALL_DIR/$BIN"
    echo "  ✓ $BIN → $INSTALL_DIR/$BIN"
  fi
done

# Instalar config de ejemplo si no existe
CONFIG_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/battos"
if [ ! -f "$CONFIG_DIR/battos.yaml" ] && [ -f "$TMP/config/battos.yaml" ]; then
  mkdir -p "$CONFIG_DIR"
  cp "$TMP/config/battos.yaml" "$CONFIG_DIR/battos.yaml"
  echo "  ✓ config → $CONFIG_DIR/battos.yaml"
fi

echo ""
echo "BattOS $VERSION instalado."
echo ""

# Verificar que INSTALL_DIR esté en PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "Para arrancar:"
    echo "  battos serve"
    ;;
  *)
    echo "Agregar al PATH (pegar en ~/.bashrc o ~/.zshrc):"
    echo "  export PATH=\"\$PATH:$INSTALL_DIR\""
    echo ""
    echo "O arrancar con ruta completa:"
    echo "  $INSTALL_DIR/battos serve"
    ;;
esac
