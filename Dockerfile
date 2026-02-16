# Build stage
FROM caddy:2.9.1-builder AS builder

# Copy plugin source files
WORKDIR /build
COPY go.mod go.sum ./
COPY *.go ./

# Build Caddy with local WhitelistPlus plugin
RUN xcaddy build \
    --with github.com/verncat/caddy-whitelistplus=/build

# Runtime stage
FROM caddy:2.9.1

# Copy the custom Caddy build
COPY --from=builder /build/caddy /usr/bin/caddy

# Create directory for database
RUN mkdir -p /data

# Set working directory
WORKDIR /srv

# Expose HTTP and HTTPS ports
EXPOSE 80 443

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s \
    CMD wget --no-verbose --tries=1 --spider http://localhost:2019/config/ || exit 1

# Run Caddy
CMD ["caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]
