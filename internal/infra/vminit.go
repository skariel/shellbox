package infra

import (
	"encoding/base64"
	"fmt"
	"os"
)

const (
	bastionSetupScript = `#!/bin/bash
# Security hardening
ufw allow OpenSSH
ufw --force enable

# Bastion-specific setup
mkdir -p /etc/ssh/sshd_config.d/
echo "PermitUserEnvironment yes" > /etc/ssh/sshd_config.d/shellbox.conf
systemctl reload sshd

# Create server directory
mkdir -p /opt/shellbox/
chmod 755 /opt/shellbox/`

	boxBaseScript = `#!/bin/bash
sudo apt-get update -y
sudo apt-get upgrade -y`
)

func GenerateBastionInitScript(sshPublicKey string) (string, error) {
	// Read and encode server binary
	serverBinary, err := os.ReadFile("/tmp/server")
	if err != nil {
		return "", fmt.Errorf("reading server binary: %w", err)
	}
	encodedServer := base64.StdEncoding.EncodeToString(serverBinary)

	fullScript := fmt.Sprintf(`#cloud-config
users:
- name: ${admin_user}
  ssh_authorized_keys:
  - %s
write_files:
- encoding: b64
  content: %s
  owner: root:root
  path: /opt/shellbox/server
  permissions: '0755'
runcmd:
- %s`, sshPublicKey, encodedServer, bastionSetupScript)

	return base64.StdEncoding.EncodeToString([]byte(fullScript)), nil
}

func GenerateBoxInitScript() (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(boxBaseScript)), nil
}
