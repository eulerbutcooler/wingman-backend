ALTER TABLE users ADD CONSTRAINT users_rank_check CHECK(rank IN ('officer','agniveer'));
