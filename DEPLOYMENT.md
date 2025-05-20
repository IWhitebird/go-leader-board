# Deployment Guide for Go Leaderboard Service

This document provides detailed instructions for deploying the Go Leaderboard Service in a production environment.

## Single-Instance Deployment (4-core machine)

The standard deployment on a 4-core machine is sufficient for most use cases, handling up to:
- 10,000 score submissions/second
- 5,000 leaderboard reads/second
- 1 million users per game
- Hundreds of distinct games

### System Requirements

#### Recommended Hardware
- **CPU**: 4 cores (minimum)
- **Memory**: 8GB RAM
  - 4GB for the application
  - 2GB for PostgreSQL
  - 2GB for OS and overhead
- **Storage**:
  - 10GB for application, logs, and WAL
  - 50GB+ for PostgreSQL (size based on expected data volume)
- **Network**: 1Gbps Ethernet

#### Software Requirements
- Linux (Ubuntu 20.04 LTS or similar)
- Go 1.16 or higher
- PostgreSQL 12 or higher
- Nginx (optional, for TLS termination and load balancing)

### Deployment Steps

#### 1. Prepare the Server

```bash
# Update the system
sudo apt update && sudo apt upgrade -y

# Install required dependencies
sudo apt install -y postgresql postgresql-contrib

# Install Go
wget https://golang.org/dl/go1.18.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.18.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

#### 2. Set Up PostgreSQL

```bash
# Create database and user
sudo -u postgres psql -c "CREATE DATABASE leaderboard;"
sudo -u postgres psql -c "CREATE USER leaderboard WITH ENCRYPTED PASSWORD 'your_password';"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE leaderboard TO leaderboard;"

# Configure PostgreSQL for performance
sudo nano /etc/postgresql/12/main/postgresql.conf
```

Add or modify the following settings:

```
# Memory settings
shared_buffers = 1GB
work_mem = 32MB
maintenance_work_mem = 256MB

# Checkpoints
checkpoint_timeout = 5min
checkpoint_completion_target = 0.9

# WAL settings
wal_level = replica

# Query optimization
random_page_cost = 1.1
effective_cache_size = 3GB
```

Restart PostgreSQL:

```bash
sudo systemctl restart postgresql
```

#### 3. Deploy the Application

```bash
# Create application directory
sudo mkdir -p /opt/leaderboard
sudo chown $USER:$USER /opt/leaderboard

# Clone repository (or copy your build artifacts)
git clone https://github.com/your-repo/go-leader-board.git /opt/leaderboard

# Build the application
cd /opt/leaderboard
go build -o leaderboard-service

# Create data directories
mkdir -p /opt/leaderboard/data/wal
```

#### 4. Create Configuration

Create a `.env` file in the application directory:

```bash
cat > /opt/leaderboard/.env << EOF
# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=8080

# PostgreSQL Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=leaderboard
DB_PASSWORD=your_password
DB_NAME=leaderboard
DB_SSLMODE=disable
EOF
```

#### 5. Create a Systemd Service

```bash
sudo nano /etc/systemd/system/leaderboard.service
```

Add the following content:

```
[Unit]
Description=Leaderboard Service
After=network.target postgresql.service

[Service]
Type=simple
User=ubuntu
WorkingDirectory=/opt/leaderboard
EnvironmentFile=/opt/leaderboard/.env
ExecStart=/opt/leaderboard/leaderboard-service
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

Enable and start the service:

```bash
sudo systemctl daemon-reload
sudo systemctl enable leaderboard
sudo systemctl start leaderboard
```

#### 6. (Optional) Set Up Nginx as Reverse Proxy

```bash
sudo apt install -y nginx

sudo nano /etc/nginx/sites-available/leaderboard
```

Add the following configuration:

```
server {
    listen 80;
    server_name leaderboard.yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

Enable the configuration and restart Nginx:

```bash
sudo ln -s /etc/nginx/sites-available/leaderboard /etc/nginx/sites-enabled/
sudo systemctl restart nginx
```

## Multi-Instance Deployment (Scaling Horizontally)

For higher throughput or reliability, you can deploy multiple instances of the service.

### Architecture

1. **Load Balancer**: Nginx or cloud provider load balancer (AWS ALB, etc.)
2. **Application Instances**: Multiple instances of the leaderboard service
3. **Shared PostgreSQL**: Central database for persistence
4. **Sharding Strategy**: Partition by game_id

### Deployment Diagram

```
                             ┌─────────────┐
                             │ Load Balancer│
                             └──────┬──────┘
                                    │
         ┌──────────────────┬──────┴───────┬──────────────────┐
         │                  │              │                  │
┌────────▼────────┐ ┌──────▼──────┐ ┌─────▼─────────┐ ┌──────▼──────┐
│  Leaderboard    │ │ Leaderboard │ │  Leaderboard  │ │ Leaderboard │
│  Service (1)    │ │ Service (2) │ │  Service (3)  │ │ Service (4) │
└────────┬────────┘ └──────┬──────┘ └──────┬────────┘ └──────┬──────┘
         │                  │              │                  │
         └──────────────────┼──────────────┼──────────────────┘
                            │              │
                     ┌──────▼──────────────▼──────┐
                     │                            │
                     │       PostgreSQL           │
                     │                            │
                     └────────────────────────────┘
```

### Sharding Strategy

For multi-instance deployments, implement sharding by game_id:

1. **Range-based sharding**: Assign specific game_id ranges to each instance
2. **Hash-based sharding**: Use consistent hashing to distribute games across instances

Example configuration for hash-based sharding:

```
Instance 1: game_id % 4 = 0
Instance 2: game_id % 4 = 1
Instance 3: game_id % 4 = 2
Instance 4: game_id % 4 = 3
```

### Load Balancer Configuration

If using Nginx as a load balancer with hash-based sharding:

```
upstream leaderboard_backend {
    hash $arg_game_id consistent;
    server instance1:8080;
    server instance2:8080;
    server instance3:8080;
    server instance4:8080;
}

server {
    listen 80;
    server_name leaderboard.yourdomain.com;

    location / {
        proxy_pass http://leaderboard_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Monitoring and Maintenance

### Key Metrics to Monitor

1. **System Metrics**:
   - CPU usage
   - Memory usage
   - Disk I/O
   - Network I/O

2. **Application Metrics**:
   - Request rate
   - Response time (p50, p95, p99)
   - Error rate
   - WAL size
   - Snapshot frequency and duration

3. **Database Metrics**:
   - Connection count
   - Query latency
   - Index usage
   - Table sizes

### Maintenance Tasks

1. **Regular Backups**:
   - PostgreSQL dumps (daily)
   - WAL file retention policy (keep for 7 days)

2. **Log Rotation**:
   - Configure logrotate for application logs

3. **Performance Tuning**:
   - Review slow queries in PostgreSQL
   - Adjust PostgreSQL and application parameters based on metrics

## Disaster Recovery

### Recovery Process

1. **Instance Failure**:
   - New instance automatically recovers from WAL
   - Recovery time < 60 seconds

2. **Database Failure**:
   - Restore from latest backup
   - In-memory state from all instances remains intact

3. **Complete System Failure**:
   - Restore PostgreSQL from backup
   - Start instances to recover from WAL

## Security Considerations

1. **Network Security**:
   - Use a firewall to restrict access
   - Place service in a private subnet
   - Use TLS for all external communication

2. **Database Security**:
   - Use strong passwords
   - Restrict network access to the database
   - Encrypt sensitive data

3. **Application Security**:
   - Implement rate limiting
   - Consider adding authentication for write operations
   - Regularly update dependencies

## Conclusion

The Go Leaderboard Service is designed to run efficiently on a single 4-core machine for most use cases. However, the architecture is flexible enough to scale horizontally for increased reliability or performance if needed.

For most deployments with up to 10,000 writes/second and 5,000 reads/second, a single instance is sufficient. As your needs grow, consider implementing the multi-instance architecture described in this guide. 