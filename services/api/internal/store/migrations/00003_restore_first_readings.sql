-- +goose Up
-- One-off repair. The first hook backfill (13 Nov 2026) re-read four thoughts
-- and stored the whole new reading, which changed the type of at least one
-- thought the user had already confirmed. This adds a version that keeps the
-- original reading and takes only the hooks and timeframe from the re-read.
-- It touches nothing once applied: the guard requires version 2 to be current.

-- +goose StatementBegin
do $$
declare
  r record;
  new_id bigint;
begin
  for r in
    select v1.*, v2.hooks as new_hooks, v2.timeframe as new_timeframe, v2.model as new_model
      from thoughts t
      join thought_interpretations v2 on v2.id = t.current_interpretation_id
      join thought_interpretations v1 on v1.thought_id = t.id and v1.version = 1
     where v2.version = 2 and v2.source = 'ai' and v2.prompt_version = 'interpret_v2'
       and v1.prompt_version = 'interpret_v1'
  loop
    insert into thought_interpretations
      (thought_id, version, source, type, title, description, desired_outcome, next_steps, confidence,
       clarification_needed, clarification_question, assumptions, model, prompt_version, timeframe, hooks)
    values
      (r.thought_id, 3, 'ai', r.type, r.title, r.description, r.desired_outcome, r.next_steps, r.confidence,
       r.clarification_needed, r.clarification_question, r.assumptions, r.new_model, 'interpret_v2',
       r.new_timeframe, r.new_hooks)
    returning id into new_id;
    update thoughts set current_interpretation_id = new_id where id = r.thought_id;
  end loop;
end $$;
-- +goose StatementEnd

-- +goose Down
-- Data repair; nothing to undo.
select 1;
