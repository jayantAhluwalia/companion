-- +goose Up
-- Schema v4: nudge lanes, send windows, a shared weekly budget and the weekly review.
-- See docs/decisions.md (14 Nov 2026) and reports/Nudge timing research evidence.md.

-- Which lane a nudge went out in, and in which window of the day. The window
-- is what the per-window success tally is counted over.
alter table nudges
  add column lane        text not null default 'quick',  -- deadline | requested | opportunity | quick
  add column send_window text;                           -- lunch | evening | offday_morning | offday_afternoon | chat | null
update nudges set send_window = 'evening'
 where send_window is null and extract(hour from sent_at at time zone 'Asia/Kolkata') >= 17;
-- Nudges sent before lanes existed: one of them was a deadline reminder, and
-- it must be recorded as such or the same deadline would be reminded twice.
update nudges n set lane = 'deadline'
  from decision_runs r
 where r.id = n.decision_run_id
   and exists (select 1
                 from jsonb_array_elements(r.scores) s,
                      jsonb_array_elements(coalesce(s->'hooks', '[]'::jsonb)) h
                where (s->>'thought_id')::bigint = n.thought_id and (h->>'critical')::boolean);

-- "Later" with a chosen time: the user said when to bring a thought back.
alter table thoughts add column remind_at timestamptz;

-- How soon the person seems to need it, read from their own words.
alter table thought_interpretations add column urgency text not null default 'whenever'; -- now | soon | whenever

alter table user_preferences
  add column weekly_budget int  not null default 5,       -- nudges per 7 days, deadlines excluded
  add column review_dow    int           default 7,       -- ISO weekday of the weekly review; null = off
  add column review_time   time not null default '19:00';

create table reviews (
  id         bigint generated always as identity primary key,
  user_id    bigint not null references users(id),
  sent_at    timestamptz not null default now(),
  item_count int not null default 0,
  tap_count  int not null default 0
);
create index reviews_user_sent_idx on reviews (user_id, sent_at desc);

-- +goose Down
drop table if exists reviews;
alter table user_preferences drop column review_time, drop column review_dow, drop column weekly_budget;
alter table thought_interpretations drop column urgency;
alter table thoughts drop column remind_at;
alter table nudges drop column send_window, drop column lane;
