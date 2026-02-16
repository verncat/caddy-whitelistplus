# caddy-whitelistplus

Caddy plugin that gates access behind an IP whitelist with Telegram-based approval flow.

See [DOCS.md](DOCS.md) for configuration reference and examples.

## Features

- **IP whitelist** stored in SQLite — zero external dependencies
- **Telegram notifications** with inline Approve / Deny buttons
- **Caddy-native** handler (`whitelistplus`) and matcher (`whitelisted`) for flexible routing
- Three response modes: `block` (403), `drop` (close connection), `placeholder` (custom HTML)

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
