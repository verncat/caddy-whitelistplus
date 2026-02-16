# WhitelistPlus — Configuration Reference

## Overview

WhitelistPlus is a Caddy module that restricts access to approved IP addresses.
When a new IP connects, the plugin:

1. Records it in a local SQLite database with status **pending**.
2. Sends a Telegram message with **Approve / Deny** buttons.
3. Blocks the request (or shows a placeholder page, or drops the connection).

Once the admin taps **Approve** in Telegram the IP is whitelisted and all
subsequent requests pass through immediately.

---

## Installation

```bash
xcaddy build --with github.com/verncat/caddy-whitelistplus=.
```

---

## Global options

Placed inside the top-level `{ }` block. Configures the shared database and
Telegram bot used by all handler / matcher instances.

```caddyfile
{
    whitelistplus {
        db_path          <path>       # SQLite file (default: whitelist.db)
        telegram_token   <token>      # Telegram Bot API token
        telegram_chat_id <id>         # Chat / group ID for notifications
        telegram_message <template>   # Custom message template (optional)
    }
}
```

| Option             | Required | Default          | Description                          |
|--------------------|----------|------------------|--------------------------------------|
| `db_path`          | no       | `whitelist.db`   | Path to the SQLite database file     |
| `telegram_token`   | no*      | —                | Bot token from @BotFather            |
| `telegram_chat_id` | no*      | —                | Numeric chat ID for approval messages|
| `telegram_message` | no       | built-in template| Custom message template (supports `{{.IP}}`, `{{.Host}}`, `{{.Path}}`, `{{.UserAgent}}`, `{{.Time}}`)|

> \* Without Telegram credentials the plugin still blocks unknown IPs but no
> notifications are sent.

---

## Handler — `whitelistplus`

HTTP middleware that enforces the whitelist. Must be ordered before the
directives it should protect:

```caddyfile
{
    order whitelistplus before basicauth
}
```

### Syntax

```caddyfile
whitelistplus {
    action             <block|drop|placeholder>
    placeholder        "<html>…</html>"
    placeholder_status <code>
}
```

Or simply `whitelistplus` (no block) to use the defaults.

| Option               | Default | Description                                      |
|----------------------|---------|--------------------------------------------------|
| `action`             | `block` | How to respond to non-approved IPs               |
| `placeholder`        | built-in page | HTML body for `placeholder` action          |
| `placeholder_status` | `403`   | HTTP status code for the placeholder response     |

#### Actions

| Action        | Behaviour                                          |
|---------------|----------------------------------------------------|
| `block`       | Returns **403 Forbidden**                          |
| `drop`        | Hijacks and closes the TCP connection silently      |
| `placeholder` | Returns a custom HTML page with the configured status |

---

## Matcher — `whitelisted`

A request matcher that is **true** when the client IP has status `approved` in
the database.

```caddyfile
@approved whitelisted
@blocked  not whitelisted
```

This lets you build arbitrary routing logic:

```caddyfile
@approved whitelisted

handle @approved {
    reverse_proxy localhost:8080
}

handle {
    whitelistplus          # sends Telegram notification on first visit
    respond "Denied" 403
}
```

---

## Full examples

### 1 — Simple: placeholder page for everyone not approved

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          /data/whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

example.com {
    whitelistplus {
        action             placeholder
        placeholder_status 403
        placeholder        <<HTML
            <!DOCTYPE html>
            <html>
            <body style="display:flex;justify-content:center;align-items:center;height:100vh;font-family:sans-serif">
            <h1>🔒 Access pending approval</h1>
            </body>
            </html>
            HTML
    }

    reverse_proxy localhost:3000
}
```

### 2 — Drop connection (stealth mode)

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          /data/whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

secret.example.com {
    whitelistplus {
        action drop
    }

    reverse_proxy localhost:8443
}
```

### 3 — Matcher-based routing (most flexible)

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          /data/whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

example.com {
    @approved whitelisted

    handle @approved {
        reverse_proxy localhost:8080
    }

    handle {
        whitelistplus
        respond "Your IP is not approved yet." 403
    }
}
```

### 4 — Protect only a subpath

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          /data/whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
    }
}

example.com {
    handle /admin/* {
        whitelistplus {
            action block
        }
        reverse_proxy localhost:9090
    }

    handle {
        reverse_proxy localhost:8080
    }
}
```

### 5 — Custom Telegram message template

```caddyfile
{
    order whitelistplus before basicauth

    whitelistplus {
        db_path          /data/whitelist.db
        telegram_token   {env.TG_BOT_TOKEN}
        telegram_chat_id {env.TG_CHAT_ID}
        telegram_message <<TMPL
🚨 *New Connection Attempt*

📍 *IP Address:* `{{.IP}}`
🌐 *Hostname:* {{.Host}}
📂 *Requested Path:* {{.Path}}
🖥️ *User-Agent:* `{{.UserAgent}}`
⏰ *Timestamp:* {{.Time}}

Please review and take action.
TMPL
    }
}

example.com {
    whitelistplus
    reverse_proxy localhost:8080
}
```

> Available template variables: `{{.IP}}`, `{{.Host}}`, `{{.Path}}`, `{{.UserAgent}}`, `{{.Time}}`

---

## SQLite schema

The plugin creates the table automatically:

```sql
CREATE TABLE IF NOT EXISTS whitelist (
    ip            TEXT PRIMARY KEY,
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | denied
    requested_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    approved_at   DATETIME,
    host          TEXT,
    path          TEXT,
    user_agent    TEXT
);
```

You can query / modify it directly:

```bash
sqlite3 /data/whitelist.db "SELECT * FROM whitelist"
sqlite3 /data/whitelist.db "UPDATE whitelist SET status='approved' WHERE ip='1.2.3.4'"
```

---

## Telegram setup

1. Create a bot via [@BotFather](https://t.me/BotFather) and copy the token.
2. Send any message to the bot (or add it to a group).
3. Get the chat ID:
   ```
   curl https://api.telegram.org/bot<TOKEN>/getUpdates | jq '.result[0].message.chat.id'
   ```
4. Set `telegram_token` and `telegram_chat_id` in the Caddyfile (use
   `{env.VAR}` placeholders to avoid hardcoding secrets).

When a new IP hits the server you will receive a message like:

```
🔒 WhitelistPlus — New Access Request

IP:   203.0.113.42
Host: example.com
Path: /
Time: 2026-02-16 12:00:00 UTC

[✅ Approve]  [❌ Deny]
```

Pressing a button updates the database instantly. The edited message confirms
the action.
