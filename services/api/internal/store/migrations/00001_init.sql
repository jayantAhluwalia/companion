-- +goose Up
-- Schema v1. Design rules (docs/02-action-plan.md § 5.2):
--   * thoughts.raw_content is never updated; understanding lives in versioned interpretations
--   * thought_events is append-only; decision_runs logs every evaluator tick, including "none"
--   * one user today, but user_id on every table so productizing is a migration, not a rewrite

create table users (
  id                bigint generated always as identity primary key,
  telegram_chat_id  bigint unique not null,
  display_name      text,
  timezone          text not null default 'Asia/Kolkata',
  created_at        timestamptz not null default now()
);

create table user_preferences (
  user_id               bigint primary key references users(id),
  quiet_start           time not null default '23:00',
  quiet_end             time not null default '07:00',
  work_start            time,
  work_end              time,
  work_days             int[] not null default '{1,2,3,4,5}',   -- ISO weekday, 1 = Monday
  days_off              int[] not null default '{6,7}',
  nudge_during_work     text not null default 'lunch' check (nudge_during_work in ('yes','lunch','no')),
  daily_limit           int not null default 1,
  min_interval_minutes  int not null default 240,
  notifications_enabled boolean not null default true,
  style_scores          jsonb not null default '{}',
  onboarded_at          timestamptz,
  updated_at            timestamptz not null default now()
);

create table thoughts (
  id                        bigint generated always as identity primary key,
  user_id                   bigint not null references users(id),
  raw_content               text not null,
  source                    text not null default 'telegram',
  status                    text not null default 'active'
                            check (status in ('active','snoozed','done','archived','faded')),
  current_interpretation_id bigint,
  snoozed_until             timestamptz,
  created_at                timestamptz not null default now(),
  updated_at                timestamptz not null default now(),
  last_touched_at           timestamptz not null default now(),
  last_nudged_at            timestamptz,
  nudge_count               int not null default 0,
  ignored_streak            int not null default 0
);
create index thoughts_user_status_idx on thoughts (user_id, status);

create table thought_interpretations (
  id                     bigint generated always as identity primary key,
  thought_id             bigint not null references thoughts(id) on delete cascade,
  version                int not null,
  source                 text not null check (source in ('ai','user')),
  type                   text not null,
  title                  text not null,
  description            text not null default '',
  desired_outcome        text not null default '',
  next_steps             jsonb not null default '[]',
  confidence             numeric(4,3) not null default 0,
  clarification_needed   boolean not null default false,
  clarification_question text not null default '',
  assumptions            jsonb not null default '[]',
  model                  text,
  prompt_version         text,
  created_at             timestamptz not null default now(),
  unique (thought_id, version)
);
alter table thoughts
  add constraint thoughts_current_interpretation_fk
  foreign key (current_interpretation_id) references thought_interpretations(id);

create table decision_runs (
  id              bigint generated always as identity primary key,
  user_id         bigint not null references users(id),
  started_at      timestamptz not null default now(),
  finished_at     timestamptz,
  trigger         text not null,                  -- ticker | manual | message
  candidate_count int not null default 0,
  scores          jsonb not null default '[]',    -- per candidate: {thought_id, R, I, A, O, F, score}
  llm_decision    jsonb,
  outcome         text not null default 'pending', -- nudged | none | blocked
  reason          text not null default ''
);

create table nudges (
  id                 bigint generated always as identity primary key,
  user_id            bigint not null references users(id),
  thought_id         bigint not null references thoughts(id) on delete cascade,
  decision_run_id    bigint references decision_runs(id),
  nudge_type         text not null,
  style              text not null,
  channel            text not null default 'telegram',
  message            text not null,
  suggested_action   text not null default '',
  status             text not null default 'scheduled', -- scheduled | sent | responded | expired | suppressed
  suppressed_reason  text,
  channel_message_id text,
  sent_at            timestamptz,
  responded_at       timestamptz,
  response_action    text,                                -- do_it | next_steps | later | not_interested
  feedback           text,                                -- helpful | not_helpful
  feedback_reason    text,                                -- tone | timing | relevance | other
  created_at         timestamptz not null default now()
);
create index nudges_user_sent_idx on nudges (user_id, sent_at desc);

create table thought_events (
  id         bigint generated always as identity primary key,
  thought_id bigint not null references thoughts(id) on delete cascade,
  nudge_id   bigint references nudges(id),
  type       text not null,
  payload    jsonb not null default '{}',
  created_at timestamptz not null default now()
);
create index thought_events_thought_idx on thought_events (thought_id, created_at);

create table llm_calls (
  id             bigint generated always as identity primary key,
  user_id        bigint references users(id),
  thought_id     bigint references thoughts(id) on delete set null,
  purpose        text not null,                 -- interpret | decide | converse
  model          text not null,
  prompt_version text not null,
  input_tokens   int not null default 0,
  output_tokens  int not null default 0,
  cost_usd       numeric(10,6) not null default 0,
  latency_ms     int not null default 0,
  stop_reason    text,
  error          text,
  created_at     timestamptz not null default now()
);

-- +goose Down
drop table if exists llm_calls, thought_events, nudges, decision_runs,
  thought_interpretations, thoughts, user_preferences, users cascade;
