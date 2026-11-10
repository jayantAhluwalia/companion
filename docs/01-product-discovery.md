# Personal AI Companion --- Product Discovery & Technical Plan

**Document version:** 0.2\
**Date:** 10 November 2026\
**Status:** Discovery in progress\
**Initial platform:** iOS\
**Initial user:** Founder / personal use\
**Notification approach:** Adaptive\
**Proposed architecture:** Hybrid intelligence (deterministic backend +
LLM)

------------------------------------------------------------------------

## 1. Executive summary

The project is a mobile-first personal AI companion that captures
spontaneous thoughts, remembers things the user wants to do, and
intelligently helps turn those intentions into action.

It is not intended to be another conventional to-do list, reminder app,
habit tracker, or productivity dashboard.

The core problem is that people often think of things they would like to
do but do not necessarily have to do. These intentions are not urgent,
may not have deadlines, and are easy to forget or postpone. Writing them
in Notes is not enough if the user never returns to those notes.

The product should let the user capture a thought in seconds and then
take responsibility for understanding, remembering, revisiting, and
helping progress it.

**Core proposition:**

> A place for your thoughts. An agent that helps you act on them.

The product's defining behaviour is not simply reminding the user that
something exists. It should help the user make progress, find a next
step, identify a useful opportunity, or consciously decide that the
intention is no longer relevant.

------------------------------------------------------------------------

## 2. Product vision and philosophy

### Vision

Build a personal AI companion that remembers the things a user wants to
do, understands why they matter, and gently helps turn those intentions
into action.

### Product principles

1.  **Capture without friction:** The user should not have to organize,
    categorize, prioritize, or schedule a thought before saving it.
2.  **Remember without requiring maintenance:** The system should retain
    and revisit intentions without requiring the user to keep checking
    an inbox.
3.  **Nudge without nagging:** Notifications should be relevant,
    contextual, low-pressure, and limited.
4.  **Progress over reminders:** Each interaction should ideally move an
    intention forward, help the user decide, or let them consciously
    drop it.
5.  **Help without taking control:** The agent can research, prepare,
    and suggest, but external actions should require user approval.
6.  **Learn gradually:** The agent should adapt to actual feedback and
    behaviour rather than assuming it knows the user's preferences.
7.  **No guilt or artificial urgency:** Not completing something should
    never be framed as failure. Not everything needs a deadline.

### What the product is not

-   A traditional task manager.
-   A calendar replacement.
-   A habit tracker built around streaks.
-   A system that turns every thought into a task.
-   A notification machine that repeatedly asks whether something is
    done.
-   An autonomous agent that takes external actions without approval.

------------------------------------------------------------------------

## 3. The problem being solved

People have many spontaneous intentions and interests, for example:

-   Sell an old motorcycle helmet.
-   Go for a long motorcycle ride.
-   Explore off-road trails.
-   Learn photography.
-   Start making gaming videos.
-   Improve mobility and strength.
-   Research a future purchase.
-   Explore a new place or experience.

These thoughts can be meaningful without being urgent. They are easy to
forget because conventional productivity tools focus on tasks,
deadlines, priorities, and schedules.

Users may capture thoughts in Notes, but that does not guarantee they
will revisit them or act on them.

### Product opportunity

Create a system that:

-   Captures the thought immediately.
-   Understands what the user meant.
-   Retains the original wording and context.
-   Determines whether and when it deserves attention.
-   Brings it back in a useful way.
-   Helps the user take the next step.
-   Learns which approaches work for that user.

------------------------------------------------------------------------

## 4. Initial product decisions

  -----------------------------------------------------------------------
  Decision                            Current direction
  ----------------------------------- -----------------------------------
  Platform                            iOS first

  Initial ambition                    Build for personal use first; keep
                                      future productization possible

  Notification approach               Adaptive

  Interaction styles                  Both thoughtful companion and
                                      proactive coach

  Capture                             Text first, voice capture included
                                      in MVP if practical

  AI approach                         Existing LLM API; no custom model
                                      training initially

  Backend                             Go

  Mobile                              React Native + Expo + TypeScript

  Database                            PostgreSQL, potentially managed
                                      through Supabase

  AI orchestration                    Simple API calls and structured
                                      outputs; no multi-agent framework
                                      initially

  Decision model                      Hybrid: backend rules and scoring,
                                      with LLM contextual evaluation

  External actions                    Suggestions and preparation first;
                                      user approval before execution
  -----------------------------------------------------------------------

These are working decisions and can be revised during prototyping.

------------------------------------------------------------------------

## 5. Core product concepts

The product should distinguish between the user's raw thoughts, the
system's understanding, the interactions it initiates, and the feedback
it receives.

### 5.1 Thoughts

A thought is the raw input captured by the user.

It can represent an intention, interest, idea, possibility, or personal
development goal. The user does not need to choose a category.

The original thought must always be preserved, even after AI
interpretation.

### 5.2 Memory

Memory is the system's structured understanding of the thought and its
history.

It may include:

-   Original text or transcript.
-   Interpreted title and description.
-   Type of thought.
-   Desired outcome.
-   Status.
-   Suggested next steps.
-   Estimated effort, where useful.
-   User-provided context.
-   AI assumptions and confidence.
-   Related thoughts.
-   Interaction and progress history.

AI assumptions must remain distinguishable from user-provided facts. The
system should not silently change the user's meaning.

### 5.3 Nudges

A nudge is an interaction initiated by the agent.

Possible types:

-   Conversational check-in.
-   Action-oriented prompt.
-   Opportunity-based suggestion.
-   Progress follow-up.
-   Thought revival.
-   Clarification or relevance check.

A nudge should be more useful than a generic reminder.

### 5.4 Feedback

Feedback is how the user communicates whether an interaction was useful
and what happened afterwards.

Signals include:

-   Helpful / not helpful.
-   Reason for feedback.
-   Progress made.
-   Completed.
-   Snoozed / later.
-   Dismissed.
-   No longer interested.
-   No response.

Ignoring a notification is ambiguous and must not automatically be
treated as negative feedback.

------------------------------------------------------------------------

## 6. Thought classification

Not every thought should become a task. A proposed initial
classification has four broad types.

### 6.1 Actionable intentions

Example: "I need to sell my old helmet."

The agent can identify the desired outcome, suggest a first step, and
help prepare the listing.

### 6.2 Experiences and interests

Example: "I want to explore more off-road trails."

The agent remembers the interest and may bring it up when a relevant
opportunity appears. It should not immediately turn the interest into a
checklist.

### 6.3 Ideas and possibilities

Example: "I have an idea for a gaming YouTube channel."

The agent can retain the idea and help develop it when the user shows
interest. It should not automatically create homework.

### 6.4 Personal development

Example: "I want to improve my mobility."

The agent can suggest small, achievable steps and revisit progress
without imposing streaks or guilt.

The classification should be editable and revisable as the user's intent
becomes clearer.

------------------------------------------------------------------------

## 7. Primary user experience

### 7.1 Home / Thought Inbox

The home screen should prioritize capture.

Proposed elements:

-   Prominent text input.
-   Voice capture.
-   Recent thoughts.
-   Latest nudge or agent activity.
-   Minimal navigation.
-   Optional home-screen quick capture widget later.

Example prompt: **"What's on your mind?"**

The user types: "I should finally start making gaming videos."

The thought is saved immediately. AI interpretation can happen
asynchronously.

No mandatory categories, dates, priorities, or forms.

### 7.2 Thought detail

Displays:

-   Original thought.
-   AI-generated interpretation.
-   Suggested next steps.
-   Current status.
-   Relevant context.
-   Interaction history.
-   Edit, snooze, archive, or mark no longer relevant actions.

### 7.3 Nudge feed / history

Shows:

-   What the agent brought up.
-   Which thought it relates to.
-   Why it was brought up.
-   Which style it used.
-   Suggested action.
-   User feedback and subsequent progress.

### 7.4 Memory / overview

A lightweight view of:

-   Active intentions.
-   Ongoing interests.
-   Ideas.
-   Recently completed items.
-   Thoughts worth revisiting.
-   Archived or abandoned thoughts.

This should not become a complex task dashboard.

### 7.5 Notification interaction

A notification should be the beginning of a useful interaction, not
merely an alert.

Example:

> Your old helmet is still waiting to be sold. Want to spend 10 minutes
> getting the listing ready? I can help with the resale price and
> description.

Possible actions:

-   Show me the next steps.
-   Let's do it.
-   Later.
-   Not interested anymore.

The exact notification actions available on iOS should be validated
during implementation.

------------------------------------------------------------------------

## 8. Agent personalities and adaptive behaviour

The product should support two communication styles. The agent chooses
between them based on the thought, context, and learned preferences.

### 8.1 Thoughtful companion

-   Friendly.
-   Curious.
-   Conversational.
-   Suggests opportunities.
-   Checks in naturally.
-   Does not pressure the user.

Example:

> Still thinking about going for a long ride? The weekend might be a
> good opportunity. Fancy planning something?

### 8.2 Proactive coach

-   Direct.
-   Action-oriented.
-   Breaks down intentions.
-   Suggests achievable next steps.
-   Helps overcome inertia without guilt.

Example:

> Let's get your helmet listed. First, take four pictures. Then we can
> check resale prices and prepare the description.

### 8.3 Other possible approaches

The agent may also use an opportunity-based or minimal approach when
appropriate.

Example:

> You mentioned wanting to try motorcycle photography. We could find a
> suitable spot for your next ride.

### 8.4 Feedback and learning

The initial system should not assume one style is always best.

It should collect feedback and observe outcomes.

Potential signals:

  Signal          Interpretation
  --------------- ----------------------------------------
  Helpful         This interaction was useful
  Not helpful     Something about the interaction failed
  Tone reason     Preference about communication style
  Action taken    Strong outcome signal
  Progress made   Positive outcome
  Snoozed         Timing may be unsuitable
  Dismissed       Possible lack of relevance
  Ignored         Ambiguous; do not assume dislike

A simple preference-scoring system is sufficient initially. Later,
preferences can be separated by thought category, timing, message
length, and interaction type.

The system should occasionally experiment with different styles, but
should not overreact to one interaction.

------------------------------------------------------------------------

## 9. Adaptive nudge decision engine

This is the most important and differentiated component of the product.

The decision engine should not ask an LLM every few minutes whether it
should send a reminder. That would be expensive, inconsistent, and
difficult to debug.

Use a hybrid approach.

### 9.1 Candidate selection

A background worker periodically reviews active thoughts and excludes:

-   Completed thoughts.
-   Archived thoughts.
-   Snoozed thoughts.
-   Recently nudged thoughts.
-   Thoughts that have exceeded notification limits.
-   Thoughts the user has explicitly marked as no longer relevant.

### 9.2 Relevance scoring

An initial scoring hypothesis:

`Score = 0.30R + 0.20I + 0.20A + 0.15O + 0.15F`

Where each variable is normalized between 0 and 1:

  Variable   Meaning
  ---------- ---------------------------------------
  R          Relevance based on age and inactivity
  I          Importance inferred from user signals
  A          Availability of a useful next action
  O          Opportunity or contextual relevance
  F          Feedback-based preference

The weights are placeholders, not validated truths. They should be tuned
through personal use.

### 9.3 LLM contextual evaluation

The backend sends a shortlist of candidate thoughts, relevant context,
and interaction history to the LLM.

The LLM can recommend:

-   Which thought deserves attention.
-   Whether to nudge or do nothing.
-   Which interaction style to use.
-   What useful next step to suggest.
-   Whether clarification is needed.

**Doing nothing must be a valid output.**

### 9.4 Deterministic policy enforcement

The backend, not the LLM, enforces:

-   Daily notification limits.
-   Quiet hours.
-   Minimum intervals between nudges.
-   Snooze rules.
-   Notification permissions.
-   Duplicate prevention.
-   Delivery scheduling.

### 9.5 Initial notification policy

Provisional defaults:

-   Maximum two proactive nudges per day.
-   Respect configured quiet hours.
-   Avoid repeatedly nudging the same thought.
-   Back off after repeated ignored nudges.
-   Allow snoozing and disabling notifications.
-   Never invent deadlines or urgency.

These defaults should be adjustable and tested.

------------------------------------------------------------------------

## 10. Technical architecture

### 10.1 Proposed stack

  -----------------------------------------------------------------------
  Layer                   Technology              Responsibility
  ----------------------- ----------------------- -----------------------
  iOS app                 React Native + Expo +   Capture, thought views,
                          TypeScript              nudge interactions

  Backend                 Go                      API, orchestration,
                                                  decision engine

  Database                PostgreSQL              Thoughts, memory,
                                                  preferences, history

  Database hosting        Supabase or managed     Managed database and
                          PostgreSQL              optional authentication

  AI                      LLM API with structured Interpretation, next
                          outputs                 steps, contextual
                                                  evaluation, nudge text

  Background processing   Go worker / scheduled   Candidate evaluation
                          jobs                    and scheduling

  Push notifications      APNs, potentially       Notification delivery
                          through Expo            
                          Notifications           

  Semantic retrieval      pgvector, later         Similarity search and
                                                  related-thought
                                                  retrieval
  -----------------------------------------------------------------------

### 10.2 High-level architecture

1.  iOS app sends a thought to the Go API.
2.  API persists the raw thought.
3.  AI processing extracts structured intent.
4.  Backend validates and stores the interpretation.
5.  Background worker periodically evaluates active thoughts.
6.  Candidate selection and scoring happen in backend code.
7.  LLM evaluates shortlisted candidates and proposes a nudge or no
    action.
8.  Backend applies notification policies.
9.  Notification service sends an eligible push.
10. User interaction is recorded.
11. Feedback and outcomes inform future decisions.

### 10.3 Keep AI and scheduling separate

The LLM should propose interpretations, next steps, and message content.

The backend should own scheduling, policy, notification limits, and
delivery.

This separation improves predictability, reliability, cost control, and
debuggability.

### 10.4 Do not overengineer

The initial product does not require:

-   Multiple microservices.
-   Multiple autonomous agents.
-   A vector database separate from PostgreSQL.
-   Fine-tuning a model.
-   A complex agent framework.
-   Always-on phone background execution.

A single Go API, a background worker, PostgreSQL, and an LLM API are
sufficient for the first version.

------------------------------------------------------------------------

## 11. AI thought processing

Example raw thought:

"I should probably sell my old LS2 helmet. It's just sitting around."

Possible structured interpretation:

``` json
{
  "type": "actionable_intention",
  "title": "Sell old LS2 helmet",
  "desired_outcome": "Sell the helmet",
  "status": "not_started",
  "deadline": null,
  "next_steps": [
    "Take photographs",
    "Research resale price",
    "Create listing"
  ],
  "estimated_effort_minutes": 15,
  "confidence": 0.94,
  "clarification_needed": false
}
```

The exact schema should be designed and versioned.

Use an LLM API with structured output / JSON schema constraints.
Validate all model responses in the backend before storing them.

The system should preserve the original thought and support ambiguity.

Example: "I should get a better helmet."

This could be a purchase intention, research interest, or passing
thought. The AI should not assume too much. It can classify with lower
confidence or ask a short clarification.

No custom model training is required for the initial implementation.

------------------------------------------------------------------------

## 12. Memory design

Start with PostgreSQL and a small number of clear entities.

### 12.1 Thought

Potential fields:

-   `id`
-   `raw_content`
-   `interpreted_title`
-   `description`
-   `category`
-   `intent_type`
-   `status`
-   `suggested_next_steps`
-   `estimated_effort`
-   `created_at`
-   `updated_at`
-   `last_nudged_at`
-   `archived_at`

### 12.2 Nudge

Potential fields:

-   `id`
-   `thought_id`
-   `message`
-   `personality`
-   `nudge_type`
-   `scheduled_at`
-   `delivered_at`
-   `feedback`
-   `feedback_reason`
-   `interaction_status`

### 12.3 User preferences

Potential fields:

-   `id`
-   `quiet_hours`
-   `daily_notification_limit`
-   `preferred_tone`
-   `notification_enabled`
-   `learned_preferences`

### 12.4 Interaction history

Potential fields:

-   `id`
-   `thought_id`
-   `nudge_id`
-   `interaction_type`
-   `metadata`
-   `created_at`

Preserve the history of AI interpretations and user corrections. Do not
silently overwrite the original meaning.

### 12.5 Semantic search later

pgvector may eventually help identify related thoughts even when they
use different words.

Example:

Earlier: "I want to get into motorcycle photography."

Later: "I saw a beautiful picture of a bike on a mountain road."

The system may identify a relationship between them.

This is not required for the first version. SQL queries, metadata, and
simple relationships should be enough initially.

------------------------------------------------------------------------

## 13. iOS and notification constraints

The iPhone app should not be expected to wake itself every hour and run
the decision engine.

iOS background execution is opportunistic, and silent push delivery is
not guaranteed.

Run decision-making on the server. Send normal push notifications
through APNs, directly or via Expo's notification infrastructure.

Initial approach:

-   Server-side scheduled evaluations.
-   Background worker running every few hours.
-   Daily candidate evaluation.
-   Push notifications when an eligible nudge is generated.
-   Local notifications for explicitly scheduled reminders.
-   User-controlled quiet hours and notification limits.

Validate notification permissions, delivery behaviour, action handling,
and app background behaviour on a physical iPhone during development.

------------------------------------------------------------------------

## 14. Thought Revival

A proposed signature feature.

The agent occasionally revisits older thoughts that have not received
attention.

Example:

> Three weeks ago, you mentioned wanting to explore off-road riding.
> Still something you'd like to do? I can help find a suitable weekend
> route.

Thought Revival should:

-   Select thoughts based on relevance, not age alone.
-   Avoid reviving something the user has dismissed.
-   Check whether the intention is still relevant.
-   Offer a useful next step.
-   Avoid creating pressure or guilt.

This is different from repeating a reminder. It is the agent actively
revisiting personal memory.

------------------------------------------------------------------------

## 15. Agent execution capabilities

The product can evolve from suggesting next steps to helping prepare or
execute them.

Example: selling a helmet.

Possible assistance:

1.  Research comparable resale prices.
2.  Suggest suitable marketplaces.
3.  Prepare a listing description.
4.  Create a photo checklist.
5.  Help publish the listing, with explicit user approval.

For the MVP, focus on generating next steps and preparing content.
External actions and integrations can come later.

The user should remain in control.

------------------------------------------------------------------------

## 16. Competitive landscape and inspiration

The project overlaps with existing products. The opportunity is not that
no one has built AI memory or proactive reminders; it is the particular
combination of low-pressure personal intentions, adaptive interaction,
and meaningful execution assistance.

The following products were identified as worth studying. Their public
descriptions should be rechecked before making implementation or
competitive claims.

### Closest products

  ------------------------------------------------------------------------
  Product                 Area of inspiration     Website
  ----------------------- ----------------------- ------------------------
  Nodo                    Proactive follow-up,    https://getnodo.ai
                          tasks and ideas,        
                          contextual nudges       

  DeepRecall              Personal memory,        https://deeprecall.app
                          proactive check-ins,    
                          companion philosophy    

  irel                    Conversational          https://irel.ai
                          assistant, follow-up    
                          through chat            

  Ocho                    Voice-first capture,    https://useocho.com
                          thought organization,   
                          resurfacing             
  ------------------------------------------------------------------------

### Other useful products

  --------------------------------------------------------------------------------------
  Product                 Area of inspiration     Website / reference
  ----------------------- ----------------------- --------------------------------------
  DayTape                 Voice journaling,       https://daytape.ai
                          summaries, action       
                          extraction              

  mindpour                Voice capture,          https://mindpour.app
                          converting messy        
                          thoughts into           
                          structured outputs      

  TwinMind                Second-brain memory and https://twinmind.com
                          contextual recall       

  Quillo                  Voice-to-task and       App Store listing; verify current
                          reminder extraction     availability

  Structured              Visual planning, inbox, https://structured.app
                          rescheduling, widgets   

  Due                     Persistent reminders,   https://www.dueapp.com
                          snoozing and            
                          notification            
                          interactions            

  Tiimo                   Visual planning and     https://www.tiimoapp.com
                          reducing overwhelm      

  Apple Reminders         Native iOS capture,     https://www.apple.com/ios/reminders/
                          Siri, widgets and       
                          system integration      
  --------------------------------------------------------------------------------------

### Competitive research caveat

This is an initial desk-research shortlist, not a hands-on product
audit. Features, pricing, platform availability, and product positioning
can change. Some products may be early access or have limited
availability.

Before borrowing a feature, install and test the relevant product where
possible. Record:

-   How quickly a thought can be captured.
-   What happens after capture.
-   How the product represents memory.
-   How and when it follows up.
-   How users respond to nudges.
-   Whether it helps with next steps.
-   How it handles ignored or abandoned intentions.
-   What feels useful versus annoying.

### Potential differentiation

The product should focus on:

**Helping people follow through on things they genuinely want to do but
do not have an obligation to do.**

Conventional productivity systems manage tasks. This product should
understand unfinished intentions.

------------------------------------------------------------------------

## 17. MVP scope

### Must have

-   iOS application.
-   Fast text capture.
-   Voice capture / speech-to-text if practical in the first iteration.
-   Persistent thought storage.
-   AI interpretation with structured outputs.
-   Thought classification.
-   Suggested next steps.
-   Thought detail and status management.
-   Background candidate evaluation.
-   Adaptive nudge generation.
-   Companion and coach styles.
-   Push notifications.
-   Helpful / not helpful feedback.
-   Basic preference learning.
-   Snooze, dismiss, complete, and no-longer-relevant actions.
-   Notification limits and quiet hours.

### Nice to have

-   Share sheet integration.
-   Home-screen widget.
-   Link and image capture.
-   Related-thought detection.
-   Contextual suggestions.
-   Agent-assisted research.
-   More detailed interaction history.

### Out of scope initially

-   Calendar integration.
-   Email integration.
-   WhatsApp integration.
-   Social features.
-   Habit tracking and streaks.
-   Complex productivity analytics.
-   Autonomous external actions.
-   Multiple specialized agents.
-   Always-on location awareness.

------------------------------------------------------------------------

## 18. Development milestones

### Iteration 1 --- Build the memory

Target: approximately first week.

-   Create the iOS app.
-   Build quick thought capture.
-   Create Go API.
-   Set up PostgreSQL.
-   Implement thought persistence.
-   Integrate basic AI interpretation.
-   Build thought detail view.

**Success:** The user can dump random thoughts and the system
understands and remembers them.

### Iteration 2 --- Build the intelligence

Target: approximately weeks two and three.

-   Build background worker.
-   Implement candidate selection.
-   Implement initial relevance scoring.
-   Generate contextual nudges.
-   Support companion and coach styles.
-   Integrate push notifications.
-   Add notification policies and quiet hours.

**Success:** The agent proactively brings relevant thoughts back.

### Iteration 3 --- Build the learning loop

Target: approximately weeks four and five.

-   Record feedback.
-   Track interaction outcomes.
-   Implement basic preference scoring.
-   Add notification backoff.
-   Add snoozing and dismissal.
-   Track progress and completion.
-   Refine the decision engine.

**Success:** The agent begins adapting to the user's behaviour.

### Personal testing

Use the app for 2--3 weeks with a varied collection of thoughts:

-   Things to buy or sell.
-   Experiences to have.
-   Skills to learn.
-   Personal improvements.
-   Things repeatedly postponed.
-   Random ideas.

Evaluate whether capture is natural, nudges are relevant, actions
increase, and notifications remain tolerable.

------------------------------------------------------------------------

## 19. Success metrics

The product should optimize for meaningful progress, not engagement for
its own sake.

### Primary metric

**Intentions converted into meaningful action.**

### Supporting metrics

-   Number of thoughts captured.
-   Percentage of thoughts interpreted correctly.
-   Percentage of nudges rated helpful.
-   Percentage of nudges followed by progress.
-   Time between capture and first meaningful progress.
-   Repeatedly ignored nudges.
-   Percentage of thoughts consciously abandoned or archived.
-   Notification opt-out and snooze behaviour.

A high number of app opens or notifications is not necessarily success.

------------------------------------------------------------------------

## 20. Estimated feasibility and effort

The product is technically feasible with current LLM APIs, ordinary
backend infrastructure, and iOS push notifications.

Indicative difficulty:

  -----------------------------------------------------------------------
  Component                           Feasibility / complexity
  ----------------------------------- -----------------------------------
  Thought interpretation              High feasibility; low-to-moderate
                                      complexity

  Persistent memory                   High feasibility; low complexity

  Suggested next steps                High feasibility; low-to-moderate
                                      complexity

  Candidate selection                 High feasibility; moderate design
                                      work

  Contextual nudge decision           Feasible; moderate complexity

  Nudge generation                    High feasibility

  Feedback-based personalization      Feasible; moderate complexity

  Real-world opportunity detection    Feasible but harder; requires
                                      integrations and context

  Reliable motivation / behaviour     Cannot be guaranteed; requires
  change                              real-world experimentation
  -----------------------------------------------------------------------

### Indicative development estimate

For one developer working consistently:

  Component                         Estimated time
  ------------------------------- ----------------
  iOS foundation and capture             3--4 days
  Backend and database                   2--3 days
  AI thought processing                  2--3 days
  Thought and memory management          2--3 days
  Decision engine                        3--5 days
  Nudge generation                       2--3 days
  Notifications                          2--3 days
  Feedback and personalization           3--4 days
  Testing and refinement                 4--6 days

**Total indicative estimate: 3--5 weeks** for a usable personal MVP,
assuming a focused scope and limited integrations. This is a planning
estimate, not a guarantee.

The capture and interpretation prototype can be built much sooner.
Adaptive behaviour needs real usage and iteration.

------------------------------------------------------------------------

## 21. Main technical risks and mitigations

### Risk: The agent misunderstands a thought

Mitigation: - Preserve raw input. - Use structured outputs. - Validate
model responses. - Track confidence. - Allow user correction. - Ask
clarifying questions only when necessary.

### Risk: Notification fatigue

Mitigation: - Hard backend limits. - Quiet hours. - Minimum intervals. -
Backoff after ignored nudges. - Explicit snooze and dismissal. - Make
no-op a valid decision.

### Risk: The agent makes incorrect assumptions

Mitigation: - Separate user-provided facts from AI inferences. - Do not
invent deadlines or context. - Make interpretations editable. - Use
conservative confidence thresholds.

### Risk: Personalization learns the wrong lesson

Mitigation: - Treat ignored notifications as ambiguous. - Separate
feedback from actual outcomes. - Learn across multiple interactions. -
Keep user controls available. - Allow occasional experimentation.

### Risk: iOS background limitations

Mitigation: - Run scheduling and decision-making on the server. - Use
APNs for delivery. - Do not rely on periodic client-side execution.

### Risk: Excessive architecture too early

Mitigation: - One Go backend. - One worker. - One PostgreSQL database. -
One LLM provider. - No multi-agent framework or separate vector database
initially.

------------------------------------------------------------------------

## 22. Immediate next steps

The recommended order is:

1.  Finalize the product behaviour and edge cases.
2.  Create a clickable iPhone prototype.
3.  Validate the capture and nudge experience.
4.  Finalize the database schema and API contracts.
5.  Build Iteration 1: memory and capture.
6.  Build Iteration 2: adaptive nudges.
7.  Build Iteration 3: feedback and personalization.
8.  Use it personally and refine it.

### Open product questions

These remain to be explored before implementation:

-   Should the agent proactively revive interests, or only follow up on
    explicit intentions?
-   How should it distinguish a passing thought from a meaningful
    intention?
-   Should the agent ever ask the user to clarify a thought?
-   How should it handle multiple related thoughts?
-   What makes an opportunity relevant enough to interrupt the user?
-   How much context should the agent use, and which integrations are
    worth adding?
-   Should the user have a daily digest, in addition to contextual
    nudges?
-   How should the agent decide that an intention is stale and ask
    whether to archive it?

------------------------------------------------------------------------

## 23. Working product statement

**An AI companion that remembers what you want to do, understands what
matters to you, and gently helps you turn those intentions into
action.**

The product should feel less like maintaining a to-do list and more like
having a thoughtful assistant who remembers the things you mention in
passing---and knows when and how to bring them back.

The first version should prove one thing:

**Can this system help its user act on more of the things they genuinely
want to do, without becoming another source of noise?**
