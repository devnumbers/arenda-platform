-- Server-side record of the info popups each user has already seen: a row
-- means the popup must not be shown again. The set of active popup keys lives
-- in the application code (internal/popups), so the key is plain TEXT and
-- there is no backfill.
CREATE TABLE user_popup_views (
    user_id   uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    popup_key text        NOT NULL,
    seen_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, popup_key)
);
