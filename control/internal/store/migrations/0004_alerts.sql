-- Sprint 5: alert channels (Slack, Mattermost, Discord, Teams, Telegram, webhook) and history.

CREATE TABLE alert_channels (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL UNIQUE,
    kind         TEXT NOT NULL,
    target       TEXT NOT NULL,                  -- sealed JSON (webhook URL, bot token, secret …)
    target_hint  TEXT NOT NULL DEFAULT '',       -- masked display value
    enabled      INTEGER NOT NULL DEFAULT 1,
    min_severity TEXT NOT NULL DEFAULT 'warning',
    events       TEXT NOT NULL DEFAULT '[]',
    created_at   TIMESTAMP NOT NULL,
    updated_at   TIMESTAMP NOT NULL
);

CREATE TABLE alert_history (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    ts       TIMESTAMP NOT NULL,
    rule     TEXT NOT NULL,
    severity TEXT NOT NULL,
    status   TEXT NOT NULL,                      -- firing | resolved | test
    alert_key TEXT NOT NULL DEFAULT '',
    title    TEXT NOT NULL,
    message  TEXT NOT NULL DEFAULT '',
    results  TEXT NOT NULL DEFAULT '[]'
);
CREATE INDEX alert_history_ts ON alert_history(ts);

INSERT INTO meta(key, value) VALUES ('alert_settings', '');
