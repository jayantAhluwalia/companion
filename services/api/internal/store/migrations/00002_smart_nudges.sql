-- +goose Up
-- Schema v2: smart nudges.
--   * interpretations carry "hooks": the outside-world conditions that make a thought timely
--   * signals caches what the engine found when it checked a hook that costs money (web lookups)
--   * the user's home city anchors weather and web lookups
--   * users.pending is the one piece of conversational state: which typed reply the bot awaits

alter table user_preferences
  add column home_city    text,
  add column home_country text,              -- ISO 3166-1 alpha-2, for web search locality
  add column home_lat     double precision,
  add column home_lon     double precision;

alter table users add column pending jsonb;

alter table thought_interpretations
  add column timeframe text  not null default '',
  add column hooks     jsonb not null default '[]';

create table signals (
  id          bigint generated always as identity primary key,
  thought_id  bigint not null references thoughts(id) on delete cascade,
  hook_key    text not null,                 -- hash of the hook, so a re-interpretation invalidates it
  kind        text not null,                 -- web (weather is free and cached in memory)
  state       text not null,                 -- fired | waiting | negative | unknown
  summary     text not null default '',
  facts       jsonb not null default '{}',
  fetched_at  timestamptz not null default now(),
  expires_at  timestamptz not null,
  unique (thought_id, hook_key)
);

alter table llm_calls add column web_searches int not null default 0;

-- +goose Down
alter table llm_calls drop column web_searches;
drop table if exists signals;
alter table thought_interpretations drop column hooks, drop column timeframe;
alter table users drop column pending;
alter table user_preferences drop column home_lon, drop column home_lat, drop column home_country, drop column home_city;
