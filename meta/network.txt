# Shellbox Network Architecture

## Resource Group: shellbox-infra

### Virtual Network: shellbox-network (10.0.0.0/8)

#### Bastion Subnet: bastion-subnet (10.0.0.0/24)
- **Network Security Group: bastion-nsg** (subnet level)
  - Inbound Rules:
    - Allow TCP 22 from Internet
    - Allow TCP 443 from Internet
    - Deny all other inbound traffic
  - Outbound Rules:
    - Allow all to boxes-subnet
    - Allow all to Internet
    - Deny all other outbound traffic

#### Boxes Subnet: boxes-subnet (10.1.0.0/16)
- **Box Virtual Machines**
  - Network Interface: box-nic-{n}
    - Network Security Group: box-nsg-{n} (NIC level)
      - Inbound Rules:
        - Allow TCP 22 from bastion-subnet (10.0.0.0/24)
        - Deny all other inbound traffic
      - Outbound Rules:
        - Allow all to Internet
        - Deny all to boxes-subnet (10.1.0.0/16)
        - Deny all to bastion-subnet (10.0.0.0/24)

## Security Design

### Traffic Flow
1. Users connect via SSH/HTTPS to bastion host
2. Bastion host forwards SSH connections to appropriate box
3. Boxes can only receive SSH from bastion
4. Boxes can only make outbound internet connections
5. No lateral movement between boxes possible
6. No reverse connections to bastion possible

### Network Isolation
- All resources contained within single resource group
- Bastion NSG applied at subnet level for centralized management
- Box NSGs applied at NIC level for granular control
- No subnet overlapping
- Network design supports up to ~65k boxes

### Security Principles
- Principle of least privilege applied to all NSG rules
- Clear separation between bastion and box responsibilities
- One-way SSH traffic flow enforced at network level
- No direct internet access to boxes
- All external traffic must pass through bastion