# Deployment Guide

## Docker Compose (Local / Dev)

`docker-compose.yml` defines the full stack: infrastructure components,
all six backend services, and an nginx reverse proxy. All services are
built from source using their individual Dockerfiles.

```bash
# Generate RSA keys (one-time)
make generate-keys

# Start everything
make up

# Verify all containers are running
docker compose ps

# Follow logs
make logs

# Tear down
make down

# Tear down and delete all volumes (wipes data)
docker compose down -v
```

---

## Service Ports

| Service | Host Port | Notes |
| --- | --- | --- |
| nginx (gateway) | 8080 | Single entry point for all backend APIs |
| iam | 8001 | Auth, users, orgs, workspace members, API keys |
| device-registry | 8002 | Devices, channels, fields |
| ingestion | 8003 | Telemetry write endpoint |
| query-realtime | 8004 | Historical query + WebSocket/SSE + CSV export |
| alert-notification | 8005 | Alert rules + notifications |
| normalization | 8006 | Raw → normalised pipeline worker (health check only) |
| supporting | 8007 | Video + audit (filtering + CSV export) |
| PostgreSQL | 5433 | |
| Redis | 6379 | |
| InfluxDB | 8086 | UI available at `http://localhost:8086` |
| Kafka | 9092 | |
| ZooKeeper | 2181 | Required by Kafka |

The frontend dev server runs separately on port `5173` (`npm run dev`).
In production, serve the built frontend via nginx or a CDN and point
`VITE_API_URL` at the backend gateway URL.

---

## Environment Variables Reference

### iam

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8001` | no |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | yes |
| `REDIS_ADDR` | `localhost:6380` | yes |
| `REDIS_PASSWORD` | `` | no |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `JWT_PRIVATE_KEY_PATH` | `keys/private.pem` | yes |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | yes |
| `JWT_ISSUER` | `greenlab-identity` | no |
| `LOG_LEVEL` | `info` | no |

### device-registry

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8002` | no |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | yes |
| `REDIS_ADDR` | `localhost:6380` | yes |
| `REDIS_PASSWORD` | `` | no |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | yes |
| `LOG_LEVEL` | `info` | no |

### ingestion

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8003` | no |
| `REDIS_ADDR` | `localhost:6380` | yes |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `LOG_LEVEL` | `info` | no |

### normalization

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8006` | no |
| `INFLUXDB_URL` | `http://localhost:8086` | yes |
| `INFLUXDB_TOKEN` | *(none)* | yes |
| `INFLUXDB_ORG` | `greenlab` | yes |
| `INFLUXDB_BUCKET` | `telemetry` | yes |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `LOG_LEVEL` | `info` | no |

### query-realtime

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8004` | no |
| `INFLUXDB_URL` | `http://localhost:8086` | yes |
| `INFLUXDB_TOKEN` | `my-super-secret-token` | yes |
| `INFLUXDB_ORG` | `greenlab` | yes |
| `INFLUXDB_BUCKET` | `telemetry` | yes |
| `REDIS_ADDR` | `localhost:6380` | yes |
| `REDIS_PASSWORD` | `` | no |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | yes |
| `LOG_LEVEL` | `info` | no |

### alert-notification

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8005` | no |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | yes |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `SMTP_HOST` | `smtp.example.com` | yes |
| `SMTP_PORT` | `587` | yes |
| `SMTP_USERNAME` | `` | no |
| `SMTP_PASSWORD` | `` | no |
| `SMTP_FROM` | `noreply@greenlab.io` | no |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | yes |
| `LOG_LEVEL` | `info` | no |

### supporting

| Variable | Default | Required |
| --- | --- | --- |
| `PORT` | `8007` | no |
| `DSN` | `postgres://greenlab:greenlab@localhost:5433/greenlab?sslmode=disable` | yes |
| `KAFKA_BROKERS` | `localhost:9092` | yes |
| `AWS_REGION` | `us-east-1` | yes |
| `S3_BUCKET` | `greenlab-video` | yes |
| `JWT_PUBLIC_KEY_PATH` | `keys/public.pem` | yes |
| `LOG_LEVEL` | `info` | no |

---

## Health Check Endpoints

Every service exposes a `/health` endpoint returning HTTP 200.

```bash
curl http://localhost:8001/health   # iam
curl http://localhost:8002/health   # device-registry
curl http://localhost:8003/health   # ingestion
curl http://localhost:8004/health   # query-realtime
curl http://localhost:8005/health   # alert-notification
curl http://localhost:8007/health   # supporting
curl http://localhost:9080/health   # nginx gateway
```

Infrastructure health checks configured in `docker-compose.yml`:

| Component | Check |
| --- | --- |
| PostgreSQL | `pg_isready -U greenlab` |
| Redis | `redis-cli ping` |
| Kafka | `kafka-broker-api-versions --bootstrap-server localhost:9092` |

---

## Kafka Topics Reference

| Topic | Producer | Consumers | Retention |
| --- | --- | --- | --- |
| `raw.sensor.ingest` | ingestion | normalization | 7 days |
| `normalized.sensor` | normalization | query-realtime, alert-notification | 7 days |
| `alert.events` | alert-notification (rule engine) | alert-notification (dispatcher) | 7 days |
| `user.events` | iam | supporting | 7 days |

Topics are auto-created with `KAFKA_AUTO_CREATE_TOPICS_ENABLE=true`.
For production, pre-create topics with explicit partition counts:

```bash
kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic raw.sensor.ingest \
  --partitions 12 \
  --replication-factor 3

kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic normalized.sensor \
  --partitions 12 \
  --replication-factor 3

kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic alert.events \
  --partitions 6 \
  --replication-factor 3

kafka-topics --create \
  --bootstrap-server localhost:9092 \
  --topic user.events \
  --partitions 6 \
  --replication-factor 3
```

---

## Production Considerations

### TLS

All services should sit behind a TLS-terminating reverse proxy (nginx,
Caddy, or a cloud load balancer). The services themselves do not handle
TLS.

### RSA Keys

- Generate a 4096-bit RSA key pair with `make generate-keys`.
- Store `private.pem` only in the `iam` service's secret store
  (e.g. Kubernetes Secret, AWS Secrets Manager).
- Distribute `public.pem` to all other services as a read-only mount.
- Never commit key files to version control.

### Scaling Guidance

| Service | Stateless | Scale-out Notes |
| --- | --- | --- |
| `iam` | Yes | Scale horizontally; Redis session cache is shared |
| `device-registry` | Yes | Scale horizontally; Redis API key cache is shared |
| `ingestion` | Yes | High-throughput path — scale aggressively; Kafka absorbs bursts |
| `query-realtime` | No (WS hub) | WebSocket clients are pinned to an instance; use sticky sessions or shared pub/sub when scaling beyond one replica |
| `alert-notification` | Partial | Rule engine state is in Postgres; multiple replicas need coordinated consumer groups |
| `supporting` | Yes | Scale horizontally |

### Database

- Use a managed PostgreSQL service (RDS, Cloud SQL, Supabase) with
  automated backups.
- Run database migrations before deploying new service versions.
- `ingestion` and `query-realtime` do not use PostgreSQL.

### InfluxDB

- Use InfluxDB Cloud or a dedicated instance with replication.
- Set appropriate retention policies on the `telemetry` bucket based on
  storage budget.

### Kafka

- Minimum 3 brokers with replication factor 3 for production.
- Set `KAFKA_AUTO_CREATE_TOPICS_ENABLE=false` and pre-create topics.
- Monitor consumer group lag for `query-realtime-group` and
  `alert-notification-telemetry-group`.
