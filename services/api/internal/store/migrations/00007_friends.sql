-- +goose Up
-- Schema v7: a handful of invited users (docs/05-system-review.md, stage 1).

-- Who may use the bot, and who runs it. The first user is the owner.
alter table users
  add column role       text not null default 'member' check (role in ('owner', 'member')),
  add column invited_by bigint references users(id) on delete set null,
  add column blocked_at timestamptz;          -- Telegram reported the bot as blocked
update users set role = 'owner' where id = (select min(id) from users);

create table invites (
  code       text primary key,
  created_by bigint not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  used_by    bigint references users(id) on delete set null,
  used_at    timestamptz
);

-- Each person's thoughts are numbered from 1. The global id stays internal:
-- it would otherwise show everyone how many thoughts exist in total.
alter table thoughts add column number int;
update thoughts t set number = n.rn
  from (select id, row_number() over (partition by user_id order by id) as rn from thoughts) n
 where n.id = t.id;
alter table thoughts alter column number set not null;
create unique index thoughts_user_number_idx on thoughts (user_id, number);

-- Per-person limits, so one heavy user cannot spend everyone's budget.
alter table user_preferences
  add column monthly_budget_usd numeric(8,2) not null default 4.00,
  add column captures_per_day   int          not null default 40;
update user_preferences p set monthly_budget_usd = 8.00
  from users u where u.id = p.user_id and u.role = 'owner';

-- So one person's data can be removed completely. The model-call log keeps
-- its costs for accounting but must not keep a pointer to a deleted person.
alter table llm_calls drop constraint if exists llm_calls_user_id_fkey;
alter table llm_calls add constraint llm_calls_user_id_fkey foreign key (user_id) references users(id) on delete set null;
alter table user_preferences drop constraint if exists user_preferences_user_id_fkey;
alter table user_preferences add constraint user_preferences_user_id_fkey foreign key (user_id) references users(id) on delete cascade;
alter table thoughts drop constraint if exists thoughts_user_id_fkey;
alter table thoughts add constraint thoughts_user_id_fkey foreign key (user_id) references users(id) on delete cascade;
alter table decision_runs drop constraint if exists decision_runs_user_id_fkey;
alter table decision_runs add constraint decision_runs_user_id_fkey foreign key (user_id) references users(id) on delete cascade;
alter table nudges drop constraint if exists nudges_user_id_fkey;
alter table nudges add constraint nudges_user_id_fkey foreign key (user_id) references users(id) on delete cascade;
alter table nudges drop constraint if exists nudges_decision_run_id_fkey;
alter table nudges add constraint nudges_decision_run_id_fkey foreign key (decision_run_id) references decision_runs(id) on delete set null;
alter table reviews drop constraint if exists reviews_user_id_fkey;
alter table reviews add constraint reviews_user_id_fkey foreign key (user_id) references users(id) on delete cascade;
-- A thought points at its current interpretation, and the interpretation at
-- its thought. Deleting the thought must be able to take both.
alter table thoughts drop constraint if exists thoughts_current_interpretation_fk;
alter table thoughts add constraint thoughts_current_interpretation_fk
  foreign key (current_interpretation_id) references thought_interpretations(id) on delete set null;

-- +goose Down
alter table user_preferences drop column captures_per_day, drop column monthly_budget_usd;
drop index if exists thoughts_user_number_idx;
alter table thoughts drop column number;
drop table if exists invites;
alter table users drop column blocked_at, drop column invited_by, drop column role;
