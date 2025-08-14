#!/bin/bash
# deleta /tmp/crush se existir
if [ -d /tmp/crush ]; then
    rm -rf /tmp/crush
fi

# Remove possível link simbólico problemático
if [ -L /usr/local/bin/crush ]; then
    rm -f /usr/local/bin/crush
fi

git clone --depth=1 -b "main" "https://github.com/akaytatsu/crush.git" /tmp/crush
cd /tmp/crush

# Build para um arquivo temporário primeiro
go build -tags musl -o /tmp/crush-binary .

# Move o binário para o local final
sudo mv /tmp/crush-binary /usr/local/bin/crush
sudo chmod +x /usr/local/bin/crush

# Testa se funciona
/usr/local/bin/crush --help >/dev/null 2>&1 || true