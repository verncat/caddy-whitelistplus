# caddy-whitelistplus

Caddy plugin that gates access behind an IP whitelist with Telegram-based approval flow.

See [DOCS.md](DOCS.md) for configuration reference and examples.

## Features

- **IP whitelist** stored in SQLite — zero external dependencies
- **Telegram notifications** with inline Approve / Deny buttons
- **Caddy-native** handler (`whitelistplus`) and matcher (`whitelisted`) for flexible routing
- Four response modes: `block` (403), `drop` (close connection), `placeholder` (custom HTML), `passthrough` (continue to next handler)
- **Matcher-based routing** for maximum flexibility with approved/blocked IPs

## Quick start

```bash
# build Caddy with the plugin
xcaddy build --with github.com/verncat/caddy-whitelistplus=.
```

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          ./whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

example.com {
    whitelistplus {
        action placeholder
    }
    reverse_proxy localhost:8080
}
```

## Advanced: Matcher-based routing with passthrough

Use the `whitelisted` matcher combined with `passthrough` action for maximum flexibility:

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          ./whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

example.com {
    @approved whitelisted
    @blocked  not whitelisted

    # Approved IPs get full access
    handle @approved {
        reverse_proxy localhost:8080
    }

    # Blocked IPs: register, notify Telegram, serve custom page
    handle @blocked {
        whitelistplus {
            action passthrough  # Register IP but don't block
        }
        root * /srv/blocked
        rewrite * /blocked.html
        file_server
    }
}
```

**Action modes:**
- `block` — Return 403 Forbidden (default)
- `drop` — Silently close TCP connection
- `placeholder` — Return custom HTML page
- `passthrough` — Register IP and continue to next handler (useful with matchers)

See [DOCS.md](DOCS.md) for full configuration reference and more examples.
