# Build stage
FROM caddy:2.9.1-builder AS builder

# Build Caddy with WhitelistPlus plugin
RUN xcaddy build \
    --with github.com/veronika/caddy-whitelistplus=/app

# Runtime stage
FROM caddy:2.9.1

# Copy the custom Caddy build
COPY --from=builder /usr/bin/caddy /usr/bin/caddy

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
