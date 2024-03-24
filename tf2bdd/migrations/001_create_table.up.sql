CREATE TABLE IF NOT EXISTS player
(
    steamid    BIGINT PRIMARY KEY,
    attributes TEXT,
    last_seen  BIGINT,
    last_name  TEXT,
    author     BIGINT  default 0,
    created_on integer default 0
);
