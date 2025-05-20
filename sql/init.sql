CREATE TABLE IF NOT EXISTS scores (
    id SERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL,
    user_id BIGINT NOT NULL,
    score BIGINT NOT NULL,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_scores_game_user ON scores (game_id, user_id);
CREATE INDEX IF NOT EXISTS idx_scores_game_score ON scores (game_id, score DESC);
CREATE INDEX IF NOT EXISTS idx_scores_timestamp ON scores (timestamp); 