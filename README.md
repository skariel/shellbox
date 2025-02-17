# script to prepare image and run qemu:

```
# install qemu

echo "\$nrconf{restart} = 'a';" | sudo tee /etc/needrestart/conf.d/50-autorestart.conf
sudo apt update
sudo apt install qemu-kvm qemu-system libvirt-daemon-system libvirt-clients bridge-utils genisoimage whois libguestfs-tools -y

sudo usermod -aG kvm,libvirt $USER
sudo systemctl enable --now libvirtd


# ---

mkdir -p ~/qemu-disks ~/qemu-memory

wget https://cloud-images.ubuntu.com/releases/24.04/release/ubuntu-24.04-server-cloudimg-amd64.img
cp ubuntu-24.04-server-cloudimg-amd64.img ~/qemu-disks/ubuntu-base.qcow2
qemu-img resize ~/qemu-disks/ubuntu-base.qcow2 16G


# user-data. Current hash pssw stands for "ubuntu":
cat << 'EOF' > user-data 
#cloud-config
hostname: ubuntu
users:
  - name: ubuntu
    passwd: '$6$rounds=4096$UFg6YrZy/onJUwol$cHMc9AgqYoyEZ3FnVGnnNk8C7mSitS43yOwLAshx6l9WR4FA5he6XUviVzR2D3YNaKCzSvFov8IQH6yHOVe7f.'
    lock_passwd: false
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
package_update: true
packages:
  - openssh-server
ssh_pwauth: true
ssh:
  install-server: yes
  permit_root_login: false
  password_authentication: true
EOF

# this is how to obtain pssw hash:
# mkpasswd --method=SHA-512 --rounds=4096 'ubuntu'

cat << 'EOF' > meta-data
instance-id: ubuntu-inst-1
local-hostname: ubuntu
EOF

genisoimage -output ~/qemu-disks/cloud-init.iso -volid cidata -joliet -rock user-data meta-data

# run qemu:
sudo qemu-system-x86_64 \
   -enable-kvm \
   -m 4G \
   -mem-prealloc \
   -mem-path ~/qemu-memory/ubuntu-mem \
   -smp 4 \
   -cpu host \
   -drive file=~/qemu-disks/ubuntu-base.qcow2,format=qcow2 \
   -drive file=~/qemu-disks/cloud-init.iso,format=raw \
   -nographic \
   -nic user,model=virtio,hostfwd=tcp::2222-:22,dns=8.8.8.8
```

# other info:

ssh -p 2222 ubuntu@localhost


for ssh key auth:
```
#cloud-config
hostname: ubuntu
users:
  - name: ubuntu
    shell: /bin/bash
    sudo: ALL=(ALL) NOPASSWD:ALL
    ssh_authorized_keys:
      - ssh-rsa YOUR_PUBLIC_KEY_HERE
package_update: true
packages:
  - openssh-server
ssh:
  install-server: yes
  permit_root_login: false
```
kill qemu:
 sudo killall qemu-system-x86_64
