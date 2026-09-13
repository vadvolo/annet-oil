# SSH Key Authentication in Annet Oil

## Overview

Annet Oil supports SSH key authentication for connecting to network devices through gnetcli. The gnetcli service **automatically reads SSH configuration from `~/.ssh/config`**, allowing you to use SSH keys instead of passwords without any additional configuration.

## Configuration Methods

### Method 1: Using SSH Config File (Recommended)

Gnetcli automatically reads the SSH configuration from `~/.ssh/config`. This is the recommended approach as it leverages standard SSH configuration.

#### Steps:

1. **Generate SSH key pair** (if you don't have one):
```bash
ssh-keygen -t rsa -b 2048 -f ~/.ssh/network_rsa
```

2. **Copy the public key to your network devices**:
```bash
# For Cisco devices
ssh admin@192.168.1.1 "conf t; ip ssh pubkey-chain; username admin; key-string; <paste-public-key>; exit; exit; exit; write"

# For Juniper devices
ssh root@192.168.1.2 "set system login user admin authentication ssh-rsa \"<public-key>\"; commit"

# For generic Linux-based devices
ssh-copy-id -i ~/.ssh/network_rsa.pub admin@192.168.1.1
```

3. **Configure SSH config file**:

Create or edit `~/.ssh/config`:

```ssh
# Network device with SSH key
Host router-1
    HostName 192.168.1.1
    User admin
    Port 22
    IdentityFile ~/.ssh/network_rsa
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null

# Default for all devices in subnet
Host 192.168.*
    User admin
    Port 22
    IdentityFile ~/.ssh/network_rsa
    StrictHostKeyChecking no
    UserKnownHostsFile /dev/null
```

4. **Test the connection**:
```bash
ssh router-1 "show version"
```

### Method 2: Using SSH Agent

Gnetcli supports SSH agent for key management:

1. **Add your key to SSH agent**:
```bash
ssh-add ~/.ssh/network_rsa
```

2. **Verify the key is loaded**:
```bash
ssh-add -l
```

3. **Start gnetcli with SSH agent support**:
```bash
export SSH_AUTH_SOCK
gnetcli_server
```

### Method 3: Using Custom SSH Config Path

If you need to use a custom SSH config file location, you can specify it in the Annet Oil configuration:

```yaml
gnetcli:
  host: localhost
  port: 5051
  ssh_config_path: /path/to/custom/ssh_config  # Optional custom path
```

Or set it via environment variable:
```bash
export SSH_CONFIG=/path/to/custom/ssh_config
gnetcli_server
```

### Method 4: Mixed Authentication (Key + Password Fallback)

You can configure both SSH key and password authentication. Gnetcli will try SSH key first, then fall back to password if key authentication fails.

In `config.yaml`:
```yaml
gnetcli:
  host: localhost
  port: 5051
  login: admin
  password: fallback_password  # Used if SSH key fails
```

## Security Considerations

1. **Host Key Verification**:
   - `StrictHostKeyChecking no` disables host key verification
   - Use only in trusted networks or for testing
   - For production, maintain a proper known_hosts file

2. **Key Permissions**:
   - Private keys must have restrictive permissions:
   ```bash
   chmod 600 ~/.ssh/network_rsa
   ```

3. **Key Passphrase**:
   - Consider using passphrase-protected keys for additional security
   - Use ssh-agent to manage passphrase-protected keys

## Troubleshooting

### Check SSH key authentication:
```bash
ssh -v -i ~/.ssh/network_rsa admin@192.168.1.1
```

### Common Issues:

1. **Permission denied**:
   - Check key file permissions: `ls -la ~/.ssh/`
   - Verify public key is correctly installed on device
   - Check SSH config syntax

2. **Key not found**:
   - Verify IdentityFile path in SSH config
   - Check if key exists: `ls ~/.ssh/network_rsa*`

3. **Gnetcli not using SSH config**:
   - Ensure SSH config is in the correct location: `~/.ssh/config`
   - Check gnetcli logs for SSH connection attempts
   - Verify gnetcli version supports SSH config (v1.2.0+)

## Example Configurations

### For Docker Container

When running gnetcli in Docker, mount your SSH directory:

```yaml
services:
  gnetcli:
    image: annetutil/gnetcli:latest
    volumes:
      - ~/.ssh:/root/.ssh:ro
    environment:
      - SSH_AUTH_SOCK=/ssh-agent
    volumes:
      - /run/user/1000/keyring/ssh:/ssh-agent
```

### For Systemd Service

Edit `/etc/systemd/system/gnetcli.service`:

```ini
[Service]
User=gnetcli
Environment="HOME=/home/gnetcli"
# SSH config will be read from /home/gnetcli/.ssh/config
```

## API Usage Examples

### Using SSH key authentication via API:

When the SSH config is properly set up, you can simply use the hostname:

```bash
curl -X POST http://localhost:8181/api/v1/execute \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "router-1",
    "command": "show version"
  }'
```

### With IP address (will use default SSH config):

```bash
curl -X POST http://localhost:8181/api/v1/execute \
  -H "Authorization: Bearer your-token" \
  -H "Content-Type: application/json" \
  -d '{
    "host": "192.168.1.1",
    "command": "show interfaces"
  }'
```

## Best Practices

1. **Use separate keys for different device types or security zones**
2. **Rotate keys periodically**
3. **Use configuration management to deploy public keys to devices**
4. **Monitor SSH key usage in logs**
5. **Use jump hosts/bastions for accessing production devices**
6. **Implement key-based authentication alongside certificate-based authentication where possible**