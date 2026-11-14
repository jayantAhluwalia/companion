-- +goose Up
-- A run with no scored thoughts stored the JSON value null in decision_runs.scores
-- instead of an empty list. It satisfied "not null" but could not be restored
-- from a backup, where JSON null and SQL NULL look the same. Found by the
-- restore check on the first backup (14 Nov 2026).
update decision_runs set scores = '[]' where scores = 'null'::jsonb;
alter table decision_runs add constraint decision_runs_scores_is_list check (jsonb_typeof(scores) = 'array');

-- +goose Down
alter table decision_runs drop constraint decision_runs_scores_is_list;
