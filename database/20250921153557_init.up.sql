CREATE TABLE posts (
    id                 BIGSERIAL PRIMARY KEY,
    title              TEXT        NOT NULL,
    body               TEXT        NOT NULL,
    user_id            BIGINT      NOT NULL,
    comments_disabled  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE comments (
    id         BIGSERIAL PRIMARY KEY,
    post_id    BIGINT NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    parent_id  BIGINT REFERENCES comments(id) ON DELETE CASCADE,
    user_id    BIGINT NOT NULL,                
    body       TEXT   NOT NULL CHECK (char_length(body) <= 2000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- индекс для пагинации по реплаям
CREATE INDEX idx_comments_pagination
    ON comments (post_id, parent_id, created_at, id);

-- индекс для корневых комментариев
CREATE INDEX idx_comments_roots
    ON comments (post_id, created_at, id)
    WHERE parent_id IS NULL;
