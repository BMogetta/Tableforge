-- Adds player roles and enriches allowed_emails for admin management.

-- Role enum
CREATE TYPE player_role AS ENUM ('player', 'manager', 'owner');

-- Add role to players — default is player
ALTER TABLE players
    ADD COLUMN role player_role NOT NULL DEFAULT 'player';

-- Enrich allowed_emails: track who invited whom and with what intended role
ALTER TABLE allowed_emails
    ADD COLUMN role player_role NOT NULL DEFAULT 'player';