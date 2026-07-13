# Logging Configuration

annet-oil supports structured logging with optional S3 upload for log archival.

## Configuration

Add the `logging` section to your `config.yaml`:

```yaml
logging:
  level: "info"           # debug, info, warn, error
  format: "json"          # json or text
  output: "/var/log/annet-oil/api.log"
  s3:
    enabled: false
    bucket: "my-logs-bucket"
    prefix: "logs/annet-oil/"
    region: "us-east-1"
```

## Options

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level` | string | `info` | Minimum log level |
| `format` | string | `json` | Output format (json/text) |
| `output` | string | `""` | File path for logs (empty = stdout only) |
| `s3.enabled` | bool | `false` | Enable S3 upload |
| `s3.bucket` | string | `""` | S3 bucket name |
| `s3.prefix` | string | `""` | S3 key prefix |
| `s3.region` | string | `""` | AWS region |

## Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Detailed debugging information |
| `info` | General operational information |
| `warn` | Warning conditions |
| `error` | Error conditions |

## Output Formats

### JSON (recommended for production)

```yaml
logging:
  format: "json"
```

Output:
```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"request","method":"POST","path":"/api/v0/gen","status":200,"latency_ms":150}
```

### Text (human-readable)

```yaml
logging:
  format: "text"
```

Output:
```
time=2024-01-15T10:30:00Z level=INFO msg=request method=POST path=/api/v0/gen status=200 latency_ms=150
```

## File Logging

Write logs to a file in addition to stdout:

```yaml
logging:
  output: "/var/log/annet-oil/api.log"
```

The directory will be created if it doesn't exist.

## S3 Upload

Enable automatic log rotation and S3 upload:

```yaml
logging:
  output: "/var/log/annet-oil/api.log"
  s3:
    enabled: true
    bucket: "company-logs"
    prefix: "annet-oil/prod/"
    region: "eu-west-1"
```

### How It Works

1. Logs are written to the local file
2. Every hour (or on shutdown), logs are:
   - Rotated (renamed with timestamp)
   - Compressed with gzip
   - Uploaded to S3
   - Local rotated file is deleted

### S3 Key Format

```
{prefix}{date}/{filename}.gz
```

Example:
```
annet-oil/prod/2024/01/15/api.log.20240115-103000.gz
```

### AWS Credentials

S3 upload uses the default AWS credential chain:
1. Environment variables (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`)
2. Shared credentials file (`~/.aws/credentials`)
3. IAM role (for EC2/ECS)

Required IAM permissions:
```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject"
      ],
      "Resource": "arn:aws:s3:::company-logs/annet-oil/*"
    }
  ]
}
```

## Request Logging

Every HTTP request is logged with:

| Field | Description |
|-------|-------------|
| `request_id` | Unique request identifier |
| `method` | HTTP method |
| `path` | Request path |
| `status` | Response status code |
| `size` | Response size in bytes |
| `latency_ms` | Request duration |
| `remote_addr` | Client IP address |
| `user_agent` | Client user agent |

## Examples

### Development

```yaml
logging:
  level: "debug"
  format: "text"
  output: ""
```

### Production (Local Only)

```yaml
logging:
  level: "info"
  format: "json"
  output: "/var/log/annet-oil/api.log"
```

### Production (With S3)

```yaml
logging:
  level: "info"
  format: "json"
  output: "/var/log/annet-oil/api.log"
  s3:
    enabled: true
    bucket: "company-logs"
    prefix: "annet-oil/prod/"
    region: "eu-west-1"
```

## MCP Server Logging

The TypeScript MCP server uses `pino` for structured logging to stderr:

```bash
export LOG_LEVEL=debug  # debug, info, warn, error
```

MCP logs are separate from the Go backend logs.

## Log Analysis

### Using jq

```bash
# Filter errors
cat api.log | jq 'select(.level == "ERROR")'

# Find slow requests (>1s)
cat api.log | jq 'select(.latency_ms > 1000)'

# Count requests by path
cat api.log | jq -r '.path' | sort | uniq -c | sort -rn
```

### Using grep (text format)

```bash
# Find errors
grep 'level=ERROR' api.log

# Find specific request
grep 'request_id=abc123' api.log
```
