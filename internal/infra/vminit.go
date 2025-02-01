package infra

import (
	"encoding/base64"
	"fmt"
)

const (
	bastionSetupScript = `#!/bin/bash
sudo apt-get update -y
sudo apt-get upgrade -y

# Security hardening
ufw allow OpenSSH
ufw --force enable

# Bastion-specific setup
mkdir -p /etc/ssh/sshd_config.d/
echo "PermitUserEnvironment yes" > /etc/ssh/sshd_config.d/shellbox.conf
systemctl reload sshd

# Create shellbox directory
mkdir -p /home/\${admin_user}`

	boxBaseScript = `#!/bin/bash
sudo apt-get update -y
sudo apt-get upgrade -y`
)

func GenerateBastionInitScript() (string, error) {
	fullScript := fmt.Sprintf(`#cloud-config
runcmd:
- %s`, bastionSetupScript)

	return base64.StdEncoding.EncodeToString([]byte(fullScript)), nil
}

func GenerateBoxInitScript() (string, error) {
	return base64.StdEncoding.EncodeToString([]byte(boxBaseScript)), nil
}
