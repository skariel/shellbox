package infra

import (
	"encoding/base64"
	"fmt"
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

# Create shellbox directory
mkdir -p /home/\${admin_user}/shellbox`

	boxBaseScript = `#!/bin/bash
sudo apt-get update -y
sudo apt-get upgrade -y`
)

func GenerateBastionInitScript(sshPublicKey string) (string, error) {
	fullScript := fmt.Sprintf(`#cloud-config
users:
- name: ${admin_user}
  ssh_authorized_keys:
  - %s
runcmd:
- %s`, sshPublicKey, bastionSetupScript)

	return base64.StdEncoding.EncodeToString([]byte(fullScript)), nil
}

func GenerateBoxInitScript() (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(boxBaseScript)), nil
}
