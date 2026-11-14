-- +goose Up
-- Schema v5: hardening after the 14 Nov 2026 design review (docs/05-system-review.md).

-- A capture is identified by the Telegram message it came from, so a message
-- delivered twice (after a restart) is stored once.
alter table thoughts
  add column source_message_id  bigint,
  add column interpret_attempts int not null default 0;
create unique index thoughts_source_message_idx on thoughts (user_id, source_message_id)
  where source_message_id is not null;

-- How a review came about. Only scheduled ones count against the weekly
-- budget and the "two untouched reviews" pause.
alter table reviews add column trigger text not null default 'scheduled'; -- scheduled | manual

-- Small durable state for the process itself (the Telegram update offset).
create table kv (
  key        text primary key,
  value      text not null,
  updated_at timestamptz not null default now()
);

create index decision_runs_user_idx on decision_runs (user_id, id desc);
create index nudges_thought_idx     on nudges (thought_id);
create index llm_calls_created_idx  on llm_calls (created_at);

-- "Faded" is retired: a thought that goes unanswered twice now waits in the
-- weekly review instead of silently disappearing. Bring back any that faded.
update thoughts set status = 'active', updated_at = now() where status = 'faded';

-- +goose Down
drop index if exists llm_calls_created_idx, nudges_thought_idx, decision_runs_user_idx;
drop table if exists kv;
alter table reviews drop column trigger;
drop index if exists thoughts_source_message_idx;
alter table thoughts drop column interpret_attempts, drop column source_message_id;
