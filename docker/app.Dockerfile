
# ESS Smart API Docker Image
# Maintainer: SafeConstructors Engineering <engineering@safeconstructors.com>

# Builder
FROM golang:alpine AS builder
WORKDIR /app
COPY . .

# Combine all builder stage RUN commands into one
RUN apk add --no-cache curl tar \
    && curl -fSL -o /tmp/task.tar.gz https://github.com/go-task/task/releases/download/v3.7.0/task_linux_amd64.tar.gz \
    && tar -xzvf /tmp/task.tar.gz -C /usr/local/bin \
    && chmod +x /usr/local/bin/task \
    && rm /tmp/task.tar.gz \
    && task --verbose -t /app/src/api.Taskfile.yml build \
    && chmod +x /app/bin/run-container.sh \
    && chmod +x /app/bin/scapi

# Runner
FROM alpine:latest

# Combine installation and cleanup in one layer
RUN apk add --no-cache tzdata util-linux curl && rm -rf /var/cache/apk/*

ARG AWS_SECRET_ARN_RDS_USER
ENV AWS_SECRET_ARN_RDS_USER=${AWS_SECRET_ARN_RDS_USER}

COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/run-container.sh /app/bin/run-container.sh
COPY --from=builder /app/bin/scapi /app/scapi

ENTRYPOINT ["/app/bin/run-container.sh"]