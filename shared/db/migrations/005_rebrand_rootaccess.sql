-- Rebrand game_id from 'loveletter' to 'rootaccess' across all tables.

UPDATE rooms SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
UPDATE game_sessions SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
UPDATE game_results SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
UPDATE game_configs SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
UPDATE ratings.ratings SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
UPDATE ratings.rating_history SET game_id = 'rootaccess' WHERE game_id = 'loveletter';
