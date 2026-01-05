# Docker GPU Passthrough for vget

This guide explains how to use your host machine's NVIDIA GPU inside Docker containers for accelerated AI transcription.

## Platform Support

| Host OS | GPU | Supported |
|---------|-----|-----------|
| Windows 10/11 | NVIDIA | Yes (via WSL2) |
| Linux | NVIDIA | Yes (native) |
| macOS | Apple Silicon | No (Docker runs in VM) |
| Any | AMD/Intel | No (CUDA only) |

---

## Windows Setup (WSL2)

### Prerequisites

- Windows 10 (21H2+) or Windows 11
- NVIDIA GPU (GTX 1060+, RTX series recommended)
- WSL2 enabled
- Docker Desktop with WSL2 backend

### Step 1: Install NVIDIA Driver on Windows

Download and install the latest Game Ready or Studio driver from [nvidia.com/drivers](https://www.nvidia.com/Download/index.aspx).

> **Note:** Windows NVIDIA drivers now include WSL2 support automatically. No separate Linux driver needed.

### Step 2: Enable WSL2

```powershell
# Open PowerShell as Administrator
wsl --install
wsl --set-default-version 2
```

Restart your computer after installation.

### Step 3: Configure Docker Desktop

1. Open Docker Desktop
2. Go to **Settings** → **General**
3. Ensure **"Use the WSL 2 based engine"** is checked
4. Go to **Settings** → **Resources** → **WSL Integration**
5. Enable integration with your WSL2 distro

### Step 4: Run vget with GPU

**Using Docker Compose (recommended):**

```yaml
# compose.yml
services:
  vget:
    image: ghcr.io/guiyumin/vget:latest
    ports:
      - "8080:8080"
    volumes:
      - ./config:/home/vget/.config/vget
      - ./downloads:/home/vget/downloads
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

```bash
docker compose up
```

**Using docker run:**

```bash
docker run --gpus all -p 8080:8080 \
  -v ./config:/home/vget/.config/vget \
  -v ./downloads:/home/vget/downloads \
  ghcr.io/guiyumin/vget:latest
```

---

## Linux Setup

### Prerequisites

- Linux with kernel 5.x+
- NVIDIA GPU
- NVIDIA Driver installed
- NVIDIA Container Toolkit

### Step 1: Install NVIDIA Driver

```bash
# Ubuntu/Debian
sudo apt update
sudo apt install nvidia-driver-550  # or latest version

# Verify
nvidia-smi
```

### Step 2: Install NVIDIA Container Toolkit

```bash
# Add NVIDIA repository
curl -fsSL https://nvidia.github.io/libnvidia-container/gpgkey | sudo gpg --dearmor -o /usr/share/keyrings/nvidia-container-toolkit-keyring.gpg

curl -s -L https://nvidia.github.io/libnvidia-container/stable/deb/nvidia-container-toolkit.list | \
  sed 's#deb https://#deb [signed-by=/usr/share/keyrings/nvidia-container-toolkit-keyring.gpg] https://#g' | \
  sudo tee /etc/apt/sources.list.d/nvidia-container-toolkit.list

# Install
sudo apt update
sudo apt install nvidia-container-toolkit

# Configure Docker
sudo nvidia-ctk runtime configure --runtime=docker
sudo systemctl restart docker
```

### Step 3: Run vget with GPU

Same as Windows - use `--gpus all` or the compose file above.

---

## GPU Selection Options

| Flag | Description | Example |
|------|-------------|---------|
| `--gpus all` | Use all available GPUs | `docker run --gpus all ...` |
| `--gpus 1` | Use 1 GPU | `docker run --gpus 1 ...` |
| `--gpus "device=0"` | Use GPU at index 0 | `docker run --gpus "device=0" ...` |
| `--gpus "device=0,1"` | Use GPUs at index 0 and 1 | `docker run --gpus "device=0,1" ...` |

In compose.yml:

```yaml
deploy:
  resources:
    reservations:
      devices:
        - driver: nvidia
          count: 1              # or "all"
          device_ids: ["0"]     # specific GPU
          capabilities: [gpu]
```

---

## Verify GPU Access

Once the container is running:

```bash
# Check GPU is visible
docker exec -it <container_id> nvidia-smi
```

Expected output:

```
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 550.xx       Driver Version: 550.xx       CUDA Version: 12.x     |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
|===============================+======================+======================|
|   0  GeForce RTX 5090    Off  | 00000000:01:00.0  On |                  N/A |
+-------------------------------+----------------------+----------------------+
```

---

## Troubleshooting

### `nvidia-smi` command not found inside container

**Cause:** GPU not properly passed through to container.

**Solution:** Ensure you're using `--gpus all` flag:
```bash
docker run --gpus all ghcr.io/guiyumin/vget:latest
```

### No GPU detected in container

**Cause (Windows):** Docker Desktop not using WSL2 backend.

**Solution:**
1. Open Docker Desktop Settings
2. Enable "Use the WSL 2 based engine"
3. Restart Docker Desktop

### Permission denied / GPU not accessible

**Cause:** Outdated NVIDIA driver.

**Solution:**
- Windows: Update driver from [nvidia.com/drivers](https://www.nvidia.com/drivers)
- Linux: `sudo apt install nvidia-driver-550` (or latest)

### Container starts but GPU not used for transcription

**Cause:** vget didn't detect the GPU at startup.

**Solution:** Check container logs:
```bash
docker logs <container_id> | grep -i gpu
```

The vget server logs GPU detection status on startup.

### WSL2: "The attempted operation is not supported"

**Cause:** WSL2 GPU support requires specific Windows version.

**Solution:**
```powershell
# Check Windows version (need 21H2+)
winver

# Update WSL
wsl --update
```

---

## Performance Tips

1. **Use :cuda image** - The `:latest` image doesn't include CUDA libraries
2. **Dedicated GPU** - If you have multiple GPUs, dedicate one to Docker
3. **VRAM matters** - Larger models (whisper-large-v3) need more VRAM
4. **Monitor usage** - Run `nvidia-smi -l 1` on host to watch GPU utilization

| Model | VRAM Required |
|-------|---------------|
| whisper-tiny | ~1GB |
| whisper-small | ~2GB |
| whisper-medium | ~5GB |
| whisper-large-v3-turbo | ~6GB |
| whisper-large-v3 | ~10GB |

---

## Why No macOS GPU Support?

Docker on macOS runs inside a Linux VM (even on Apple Silicon). The VM cannot access the Metal GPU directly. For GPU-accelerated transcription on Mac, use the native vget CLI instead of Docker.

## Why No AMD/Intel GPU Support?

vget uses CUDA for GPU acceleration, which is NVIDIA-only. AMD ROCm and Intel oneAPI have limited support in whisper.cpp and are not included in vget Docker images.
