# JITAIs, Micro-Randomized Trials and Adaptive (RL / Bandit) Timing of Proactive Prompts

Notes compiled November 2026. Verification status is marked on each finding:
- **[full text]** = I read the paper's full text.
- **[abstract]** = I read the published abstract only (full text paywalled or blocked).
- **[secondary]** = figure taken from a third-party summary or a search-result excerpt; the primary source could not be opened. Treat as unconfirmed until checked.

A recurring caveat for the report writer: almost every study below tests **generic, system-authored health prompts** (walk now, log your drinks, brush your teeth) or **engagement-bait notifications from consumer apps**. None tests reminders of **a person's own previously stated intentions**, which is what the companion app sends. Transfer of effect sizes is therefore unproven in every section; the transferable material is mostly about *shape* (short-lived proximal effects, decay with repetition, weak long-term carry-over) and *method* (how to learn with little data).

---

## 1. HeartSteps V1 (Klasnja et al. 2019) and Personalized HeartSteps V2 (Liao et al. 2020)

### Takeaway
In HeartSteps V1 a walking suggestion raised the next-30-minute step count by 24% on average over six weeks, but the effect was about 107% at the start and shrank over the study; anti-sedentary suggestions had no detectable effect. HeartSteps V2 replaced fixed randomization with a Thompson-sampling algorithm that uses an informative prior from V1, a "dosage" term to penalise recent sending, and clipped send probabilities; the published evidence for it is simulation plus an uncontrolled 8-person pilot, not a causal efficacy result.

### Cited Findings

**HeartSteps V1**
- Design: 6-week MRT, 44 adults; suggestions could be delivered up to five times per day at user-selected times; at each time the app randomized whether to send a suggestion and, if so, whether it was a walking or an anti-sedentary suggestion; outcome was step count in the 30 minutes after randomization. [abstract] — [Klasnja et al. 2019, Ann Behav Med (PubMed)](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Any suggestion vs none, averaged over all days and both types: +14% in 30-minute step count (p = .06), i.e. 35 additional steps over a 253-step average. This pooled effect did not reach conventional significance. [abstract] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Any suggestion vs none at the start of the study: +66% (167 steps; p < .01), "but this effect diminished over time". [abstract] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Walking suggestion vs none, averaged over days: +24% (59 steps; p = .02). At the start of the study: +107% or 271 additional steps (p < .01), "but decreased over time". [abstract] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- "Antisedentary suggestions had no detectable effect on the 30-min step count." [abstract] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- A slide deck by Susan Murphy is reported to state that the walking suggestion "initially increases step count over succeeding 30 minutes by approximately 271 steps but by day 20 this increase is minimal". [secondary: search-result excerpt of the slides; the PDF could not be parsed, so the day number is unconfirmed] — [Murphy seminar slides](http://people.seas.harvard.edu/~samurphy/seminars/MRT1-and-1PennStateWebinar.pdf)
- Later re-analysis of V1 data motivated a "dosage" variable because "moving to contexts with larger recent dosage appears to result in smaller immediate effect of treatment and lower future rewards". [full text] — [Liao et al. 2020, arXiv 1909.03539](https://arxiv.org/abs/1909.03539)
- A 2024 functional-data re-analysis of the HeartSteps MRT models how the effect varies within the minutes after a decision point and across decision points, and by suggestion type, recent sedentariness and weekday. I read only the abstract and did not extract numeric results. [abstract] — [Yu & Qian, arXiv 2410.15049](https://arxiv.org/abs/2410.15049)

**Personalized HeartSteps (V2) algorithm**
- V2 is a 90-day trial in people with stage 1 hypertension; the RL algorithm decides at each of five user-specified times per day (roughly 2.5 hours apart) whether to send a context-tailored activity suggestion. Participants wear the tracker for one week before any suggestions start. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- The algorithm is Thompson sampling over a low-dimensional linear model of the *difference* in reward between sending and not sending; the authors chose a low-dimensional model "to trade off the bias and variance to accelerate learning". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Informative prior: built from HeartSteps V1 data, "to speed up the learning in the early phase of the study as well as to reduce the variance and diminish the impact of noisy observation". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Stated design challenge: interventions "often tend to have positive effect on the immediate reward, but likely produce negative impact on the future rewards due to user habituation and/or burden". The dosage variable (discount rate 0.95, chosen by the scientific team from V1 analyses) feeds a proxy for that delayed cost. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Clipped probabilities: the probability of sending nothing is kept at least 0.2 and the probability of sending at least 0.1 (so the send probability stays within 0.1–0.8). The stated reasons are to allow off-policy analysis afterwards and to ensure the algorithm "will continue to explore and learn, instead of locking itself into a particular policy". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Posterior and delayed-effect proxy are updated once per night, not after every decision. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Simulation evaluation (generative model built from V1, three-fold cross-validation, 96 runs per test participant): total reward beat a plain Thompson-sampling bandit for 29 of 37 participants; "average improvement of the total rewards over TS Bandit is 29.753" (units as reported by the authors). [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Pilot data: 8 participants who had been on the algorithm more than a week took on average 125 more steps in the 30-minute window after decision times than in their suggestion-free first week; 7 of 8 increased, one decreased (-38 steps). This is a within-person before/after comparison with no control. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Pilot lesson 1: for an unresponsive participant the posterior mean effect fell below zero but the send probability still ranged 0.2–0.4; the authors judged that "randomizing with this probability is likely too much". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Pilot lesson 2: a highly responsive participant's responsivity began to fall while "receiving many suggestions", and the send probability did not start to decrease until about six days later; the authors wrote "The proxy value needs improvement." [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Liao et al. describe V1 as "a 42-day physical activity study involving 37 healthy sedentary adults" with suggestions randomized "with a constant probability of 0.30"; the Klasnja abstract reports 44 adults. The 37 vs 44 difference is not explained in the text I read (37 is the count used in the simulation dataset). [full text / abstract] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539); [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)

### Inferences
- The V1 result has three separable parts that matter for design: a real but short-window effect, a strong novelty component (start-of-study effect roughly 4–5 times the six-week average for walking suggestions: 271 vs 59 steps), and content dependence (one of the two message types did nothing).
- The V2 pilot lessons point at two failure modes of an adaptive sender even with a population prior: it keeps sending to someone who is not responding because of the probability floor, and it is slow to back off when a responsive person starts to habituate.

### Gaps
- I could not open the full text of Klasnja et al. 2019 (PMC blocked automated access, Europe PMC had no open full text), so the exact randomization probabilities, the per-day decay slope and the day at which the effect became negligible are not confirmed from the primary paper. The "day 20" figure is from an unverified excerpt.
- I found no published primary efficacy result for the full HeartSteps V2/V3 trial comparing the RL policy to a non-adaptive policy. Recent HeartSteps II papers I found are idiographic modelling studies of 10–11 participants ([De La Torre et al. 2025](https://doi.org/10.1080/21642850.2025.2552479)), not policy comparisons.
- HeartSteps suggestions were system-generated activity ideas, not reminders of the user's own stated plans; transfer to the companion app is untested.

---

## 2. Other MRTs: proximal effects of prompts and their decay over weeks

### Takeaway
Across MRTs, a prompt reliably moves the immediate, low-effort outcome (opening an app, self-monitoring) and often fails to move the harder behaviour; decay over weeks is clear in HeartSteps V1, statistically undetectable in Drink Less (30 days) and JOOL (89 days), and several trials were under-powered to detect it. Where long-term engagement was measured (Drink Less), a strong proximal effect did not translate into longer retention.

### Cited Findings

**Drink Less (Bell et al. 2023) — notification effect on opening the app**
- Design: 30-day MRT in new users of an alcohol-reduction app, with two parallel arms. Each day at 8 PM MRT users were randomized 30% standard message / 30% message from a new bank / 40% no message. 349 MRT users, 10,470 person-day measurements. Primary outcome: opened the app between 8 and 9 PM. [full text] — [Bell et al. 2023, JMIR mHealth uHealth](https://mhealth.jmir.org/2023/1/e38342)
- Raw proportions opening the app in the hour: 12.2% (new message), 13.1% (standard message), 3.6% (no message). [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Effect: relative risk 3.52 (95% CI 2.91–4.25) for any notification; 3.66 (2.99–4.48) standard; 3.39 (2.77–4.13) new. Over 24 hours the effect shrank to RR 1.26 (1.19–1.34); 24-hour open rates were 35.1% / 34.2% vs 28.0%. [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Decay: estimated multiplicative change of 0.993 per day (95% CI 0.975–1.012), not significant. The trial had been powered assuming decay of 0.911 per day but recruited 598 of a planned 2000, so the time-trend analysis "were not adequately powered". [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- "Habituation" proxy (notification received the day before): standard-message effect reduced by 11% (ratio 0.889, 95% CI 0.60–1.31), new-message effect unchanged (1.013, 0.68–1.52); neither significant. "Already engaged" (opened 8–9 PM the day before): new-message effect 20% lower (0.80, 0.55–1.17), not significant. [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Long-term: median time to disengagement was 11 days (fixed daily notification), 11 days (random MRT policy) and 7 days (no notifications); log-rank P = .42, not significant. The authors conclude a fixed or random daily policy "did not lengthen the time to disengagement or increase the amount of engagement" and that notifications are better used as "dynamic components" responsive to the user's state. [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- The 8 PM send time was chosen from prior observational data showing a natural usage peak near 8 PM; the trial did not compare times of day, "Owing to the priority of not overburdening users with too many notifications sent within a day". [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)

**JOOL (Bidargaddi et al. 2018) — push notification effect on self-monitoring**
- Design: 1255 users of a commercial workplace well-being app over 89 days. At one randomly chosen time of six per day (8:30 am, 12:30 pm, 5:30, 6:30, 7:30, 8:30 pm) available users were randomized 50/50 to a push with a tailored health message or nothing. Outcome: self-monitoring in the app within the next 24 hours. [full text] — [Bidargaddi et al. 2018, JMIR mHealth uHealth](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)
- Effect: 3.9% more likely to engage in the next 24 hours (risk ratio 1.039; 95% CI 1.01–1.08; P < .05) — statistically significant but small. [full text] — [Bidargaddi et al. 2018](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)
- Decay: the effect "attenuated over the course of the study, but this effect was not statistically significant (P=.84)". [full text] — [Bidargaddi et al. 2018](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)
- Timing: weekend effect 8.7% (95% CI 1.01–1.17) vs weekday 2.5% (0.98–1.07); the weekend–weekday difference was not significant (P = .18). Largest estimated effect at 12:30 pm on weekends, 11.8% (90% CI 1.02–1.13), an exploratory finding. [full text] — [Bidargaddi et al. 2018](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)

**BariFit (Klasnja et al. 2021)**
- 51 bariatric-surgery patients, 16 weeks; SMS activity suggestions micro-randomized five times a day. "Activity suggestions had no effect." (Participants did increase activity by 1,866 steps from baseline to end of study, p < .007, in the uncontrolled pre–post comparison.) [abstract] — [Klasnja et al. 2021, Transl Behav Med](https://doi.org/10.1093/tbm/ibaa040)

**SARA (Nahum-Shani et al. 2021)**
- MRT in adolescents and emerging adults with past-month substance use; engagement strategies (an inspirational quote before the self-report window; memes/gifs after completion) were randomized. Daily self-report rate was 62.3% (n = 68). Effects depended on context: for example the reciprocity (quote) effect "significantly varied depending on whether this strategy was employed (vs. not employed) during the weekend". The reinforcement strategy "requires further consideration". The abstract gives no pooled effect size or decay estimate. [abstract] — [Nahum-Shani et al. 2021, Health Psychology](https://doi.org/10.1037/hea0001101)

**Sense2Stop**
- Published protocol only: 75 smokers, sensor-detected stressed and non-stressed minutes micro-randomized to a stress-management prompt or none, with 2.5–3 prompts per day, outcomes over the following two hours, 10 days post-quit. [abstract] — [Battalio et al. 2021, Contemp Clin Trials](https://pubmed.ncbi.nlm.nih.gov/34375749/)

**DIAMANTE (Aguilera et al. 2024) — RCT of RL-selected vs random daily texts (not an MRT)**
- 168 analysed participants with diabetes and depressive symptoms, 24 weeks, three arms: weekly mood message only (control); daily feedback and motivational texts chosen uniformly at random; daily texts whose category **and time of day (four periods between 8 AM and 8 PM)** were chosen by a Thompson-sampling contextual bandit. [full text] — [Aguilera et al. 2024, JMIR](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC11496924/)
- Adaptive arm gained 3.6 steps per day each day (95% CI 2.45–4.78; P < .001), about 608 steps cumulatively; post-intervention step increase 19% (606/3197) in the adaptive arm vs 1.6% (random) and 3.9% (control). [full text] — [Aguilera et al. 2024](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC11496924/)

**2026 MRTs**
- Cannabis-use pJITAI (MiWaves): 122 emerging adults, 30 days, twice-daily micro-randomization to message vs none with probabilities set by an RL algorithm. Sending a message increased proximal engagement (b = 0.044; 95% CI 0.018–0.070) "but did not affect proximal cannabis use" (b = 0.008; 95% CI −0.003 to 0.020). [abstract] — [Walton et al. 2026, Lancet Reg Health Am](https://doi.org/10.1016/j.lana.2026.101524)
- First-responder mindfulness MRT: 75 participants, 30 days, one of three daily conditions (none / brief text reminder / prompt to do an app-guided practice). "Results indicated no direct effect of interventions on momentary stress"; the guided-practice prompt dampened stress reactivity after a reported stressor. [abstract] — [Tran et al. 2026, Prevention Science](https://doi.org/10.1007/s11121-026-01957-5)

### Inferences
- Pattern across trials: effect size tracks how close the outcome is to the notification itself. Opening an app within the hour: RR 3.5. Self-monitoring within 24 hours: RR 1.04–1.26. Step count in 30 minutes: +14% pooled, driven by one message type and by early days. Momentary stress, proximal cannabis use, BariFit activity: no detectable effect.
- "No significant decay" in Drink Less and JOOL is weak evidence of no decay: Drink Less states it was under-powered for the time trend, and its confidence interval (0.975–1.012 per day) includes roughly a 50% loss over 30 days as well as no loss.
- DIAMANTE is the clearest controlled sign that *selection* (which message, which time) can matter more than volume: random daily texts did no better than a weekly message, while bandit-selected daily texts did.

### Gaps
- No published Sense2Stop outcome results were found; only the protocol.
- A search excerpt attributed to a SARA case study said notifications had no significant effect on survey completion in the first 50 days and a negative association in the second half; I could not tie that text to a specific paper and have not used it as a finding.
- I did not find the DIAMANTE multi-level MRT analysis results beyond the RCT outcome paper.
- None of these MRTs delivered reminders of the participant's own stated intentions with accept / defer / decline responses; I found no MRT of that prompt type.

---

## 3. The JITAI framework (Nahum-Shani et al. 2018): components, vulnerability/opportunity vs receptivity, and "provide nothing"

### Takeaway
The framework separates *when support is needed or useful* (a state of vulnerability or opportunity) from *whether the person can take it in right now* (receptivity), and treats sending nothing as a first-class intervention option rather than an absence of one. It is a design vocabulary supported by theory and examples, not by effect estimates.

### Cited Findings
- Six elements: "a distal outcome, proximal outcomes, decision points, intervention options, tailoring variables, and decision rules". [full text] — [Nahum-Shani et al. 2018, Ann Behav Med](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Decision points: "Points in time at which an intervention decision must be made." Tailoring variables: "Information concerning the individual that is used for individualization (i.e., to decide when and/or how to intervene)." Intervention options: "Array of possible treatments/actions that might be employed at any given decision point", including types, sources, modes, amounts and media of support. Decision rules: specify "which intervention option to offer, for whom, and when", linking options and tailoring variables systematically. [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- State of vulnerability/opportunity: "A period of susceptibility to negative health outcomes (vulnerability) or to positive health behavior changes (opportunity)." [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Receptivity: "the individual's transient ability and/or willingness to receive, process, and utilize just-in-time support; receptivity is a function of both internal (e.g., mood) and contextual (e.g., location) factors". It "might change rapidly in the course of a day" and depends on the type, amount and timing of support. [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Cost of ignoring receptivity: "providing support when the person is not receptive will not be beneficial and may even have negative implications on engagement with the intervention and intervention fatigue". [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Provide nothing: "The flip-side of providing the right type of support at the right time is providing nothing when the time is wrong and never providing the wrong type of support", framed as eliminating waste — any action that absorbs time or effort but adds no value or disrupts the desired process. [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Intervention fatigue is defined as "a state of emotional or cognitive weariness associated with intervention engagement", fluctuating with intervention burden, general life demands, the person's available resources (attention, mood) and illness burden. [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Worked example of a "nothing" rule: the FOCUS app prompts three times a day; "No intervention is offered if the individual ignores the prompt", treating a non-response as a signal of non-receptivity. [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- What has been learned empirically about receptivity: in a 6-week study of 189 users of a chatbot activity coach, day/time, phone battery, phone interaction, physical activity and location were associated with in-the-moment receptivity, and age, personality and device type with overall receptivity. [abstract] — [Künzler et al. 2019, IMWUT](https://doi.org/10.1145/3369805)
- Deploying receptivity models live: messages timed by machine-learning models gave "up to a 40% improvement in receptivity as compared to the control model" that sent at random times, and receptivity to the adaptive (per-user, continuously updated) model "increased over the course of the study". [abstract] — [Mishra et al. 2021, IMWUT](https://arxiv.org/abs/2011.08302)
- In MRT-era algorithms "nothing" is protected explicitly: HeartSteps V2 guarantees at least a 0.2 probability of sending nothing at every decision point. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)

### Inferences
- For the companion app the two framework concepts map onto two different questions: is this stored intention worth raising now (opportunity), and is the person able to act on a message now (receptivity). The framework treats them as distinct tailoring variables; a prompt can be well-targeted on one and wrong on the other.
- The framework's own example (FOCUS) uses an ignored prompt as evidence of non-receptivity and withholds follow-up; that is a rule-based use of exactly the kind of button/non-response signal the app collects.

### Gaps
- Nahum-Shani et al. 2018 gives no effect estimate for including a "provide nothing" option; the argument is conceptual. I found no trial that isolates the benefit of adding a nothing option beyond the dose and decay findings in sections 1, 2 and 4.
- Receptivity studies above measure response to the message (replying, response delay), not completion of the suggested behaviour.

---

## 4. Treatment fatigue, habituation and dosage caps

### Takeaway
Caps on prompts per day in JITAI studies are mostly set by investigator judgement about burden rather than by tested dose-response; the supporting evidence is indirect (declining effects with accumulated dose in HeartSteps, a null long-term effect of daily notifications in Drink Less) and one small trial found that daily notifications did not deter use compared with one every three days.

### Cited Findings
- Caps actually used: HeartSteps V1 — five decision points a day at user-selected times ([Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)); HeartSteps V2 — five a day, send probability clipped to 0.1–0.8 ([Liao et al. 2020](https://arxiv.org/abs/1909.03539)); Sense2Stop — 2.5–3 stress-management prompts a day ([Battalio et al. 2021](https://pubmed.ncbi.nlm.nih.gov/34375749/)); JOOL — at most one randomization a day, sent with probability 0.5 ([Bidargaddi et al. 2018](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)); Drink Less — at most one a day at 8 PM, sent with probability 0.6 ([Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)); MiWaves — two randomizations a day ([Walton et al. 2026](https://doi.org/10.1016/j.lana.2026.101524)); Oralytics — 140 decision times per participant in total ([Trella et al. 2024](https://arxiv.org/abs/2409.02069)); FOCUS — three prompts a day ([Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)).
- Rationale for budgets: "to avoid over burdening users, there is often a constraint on the number of treatments that should be provided per time interval (e.g., day or week). Yet there may be many more times at which the user is predicted or detected to be at risk and/or receptive." The proposed method spreads a fixed budget uniformly across eligible moments. [abstract] — [Liao et al. 2018, "Just-in-Time but Not Too Much", IMWUT](https://doi.org/10.1145/3287057)
- Evidence that accumulated dose reduces effect: in HeartSteps V1, higher recent dosage "appears to result in smaller immediate effect of treatment and lower future rewards"; and effects shrank from +107% to a +24% six-week average for walking suggestions. [full text / abstract] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539); [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Evidence from a single user under an adaptive policy: responsivity of a highly responsive HeartSteps V2 pilot participant fell while "receiving many suggestions", before the algorithm reduced its send probability. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Daily notifications without long-term benefit: in Drink Less, a fixed daily notification and a random 60%-of-days policy gave the same median time to disengagement (11 days) as each other and were not significantly different from no notifications (7 days; P = .42). The authors warn that "consistent notifications could overburden or annoy a user". [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Counter-evidence on frequency: in a 77-person exploratory trial of a stress-management app, groups receiving one notification per 24 hours viewed (d = .47 intelligent, .44 daily) and actioned (d = .50, .43) more notifications than a group receiving one per 72 hours; group had "no meaningful effects on percentage of notifications viewed or usage" of the app. The authors conclude "frequent notifications may encourage greater exposure to intervention content without deterring engagement". The trial was not powered for between-group tests. [abstract + full-text methods] — [Morrison et al. 2017, PLoS One](https://journals.plos.org/plosone/article?id=10.1371/journal.pone.0169162)
- Content rotation as a habituation counter-measure in industry: Duolingo found that new notification templates are especially effective and that the effect wears off with repetition, and built a recency penalty into its bandit so that recently seen templates are demoted. [blog, primary company source] — [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/)
- Drink Less tested the same idea in an MRT: a new, varied message bank vs one standard repeated message gave similar effects (RR 3.39 vs 3.66), and the point estimate of day-on-day decay was larger for the standard message, but not significantly. [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)

### Inferences
- All caps above are per-day figures of roughly 1–5 for health apps. The companion app's stated volume (3–10 per week) sits at or below the bottom of that range, comparable to the "occasional" arm in Morrison et al. and the Drink Less policy.
- The evidence supports "effect per prompt falls as recent dose rises" better than it supports any specific numeric cap. No source I read tested a cap value against alternatives.

### Gaps
- I found no MRT that randomized daily or weekly dose (number of prompts) as a factor and reported a dose-response curve for behaviour or retention.
- No evidence was found on habituation to reminders about self-chosen goals specifically.

---

## 5. RL and bandit methods with small data: MyBehavior, IntelligentPooling, Oralytics and related

### Takeaway
Research groups working at 2–5 decisions per user per day consistently conclude that a single user's data is too little and too noisy to learn from alone, and respond with informative priors, pooling across users and very small models; reported gains are mostly in simulation or small pilots, with DIAMANTE the main controlled trial showing a behavioural benefit of a bandit over random messaging.

### Cited Findings
- MyBehavior (first trial): 17 participants randomized to bandit-personalized suggestions (n = 9) or professionally written generic suggestions (n = 8) for 3 weeks. Personalized group "walked significantly more than the control group" (P = .05); food-choice difference not significant (P = .15); users rated personalized suggestions more positively (P < .001). The suggestions were built from the user's *own* logged frequent behaviours using an exploit–explore strategy. [full text] — [Rabbi et al. 2015, JMIR mHealth uHealth](https://doi.org/10.2196/mhealth.4160)
- MyBehavior used EXP3, a context-free multi-armed bandit. [full text, as described by a later paper] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- MyBehavior (second study): reported as 16 participants over 14 weeks, with users following more suggestions and achieving more calorie loss than in the control condition. [secondary: search excerpt of the paper] — [Rabbi et al., UbiComp 2015](https://pac.cs.cornell.edu/pubs/ubicomp_mybehavior.pdf)
- IntelligentPooling: Thompson sampling with a mixed-effects (hierarchical Bayesian) model so that each user's policy borrows from other users in proportion to how similar users are; motivated by the observation that noise from in-situ delivery "can cripple the ability of an algorithm to learn when given access to only a single user's data". [full text] — [Tomkins et al., arXiv 2002.09971](https://arxiv.org/abs/2002.09971); journal version [Machine Learning 2021](https://link.springer.com/10.1007/s10994-021-05995-8)
- IntelligentPooling result: "an average of 26% lower regret than state-of-the-art across all generative models" in simulations built from HeartSteps V1 data. In a bi-modal simulated population (one group helped by messages, one harmed), fraction of times treatment was sent to the group that should and should not get it: full pooling 0.49 / 0.46, person-specific 0.65 / 0.49, IntelligentPooling 0.59 / 0.36. [full text] — [Tomkins et al.](https://arxiv.org/abs/2002.09971)
- IntelligentPooling live pilot: deployed for ten users after an initial ten, up to five decisions a day for 90 days each. Evidence of personalization was descriptive: the gap between the pooled and personalized treatment probabilities grew with the amount of data from a user (Pearson r = .56, p < .1). No behavioural outcome comparison was reported. [full text] — [Tomkins et al.](https://arxiv.org/abs/2002.09971)
- Oralytics: each participant contributes 140 decision times, which the authors describe as "a small amount of data collected per participant"; because of this they ran a *full-pooling* Thompson-sampling algorithm with one shared linear reward model, an informative prior, and a dosage feature. [full text] — [Trella et al. 2024, arXiv 2409.02069 (AAAI 2025)](https://arxiv.org/abs/2409.02069)
- Oralytics post-trial re-evaluation in simulation (500 repetitions): full pooling vs no pooling gave mean outcome 69.724 vs 69.375 and first-quartile 43.049 vs 43.024 — full pooling "slightly higher". The authors also report the algorithm appeared to learn at least one state in which sending a prompt was effective. [full text] — [Trella et al. 2024](https://arxiv.org/abs/2409.02069)
- Oralytics operations: the RL service was unavailable on two occasions during the trial and participants received a fallback schedule. [secondary: search excerpt of the monitoring paper] — [Trella et al., npj Digital Medicine 2026](https://www.nature.com/articles/s41746-026-02669-4)
- HeartSteps V2: Thompson sampling on a low-dimensional action-centred model with a V1-derived prior; beat a plain TS bandit for 29 of 37 simulated participants. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- DIAMANTE: Thompson-sampling contextual bandit choosing message category and one of four daily time periods; 19% step increase vs 1.6% for uniform-random messaging over 24 weeks (n = 168 across three arms). [full text] — [Aguilera et al. 2024](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC11496924/)
- MiWaves: RL-set message probabilities, 122 participants, 30 days; primary outcomes were feasibility and acceptability; no comparison against a non-RL policy is reported in the abstract. [abstract] — [Walton et al. 2026](https://doi.org/10.1016/j.lana.2026.101524)

### Inferences
- Sample sizes at which these teams still chose to pool or lean on priors: HeartSteps V2 ≈ 450 decision points per user (5/day × 90 days); Oralytics 140 per user. At 3–10 nudges a week the companion app would take roughly 14–47 weeks to reach Oralytics' per-user count and roughly 45–150 weeks to reach HeartSteps V2's. (Arithmetic on cited figures.)
- The reported gains are of three different evidential grades: simulated regret (IntelligentPooling, HeartSteps V2), descriptive pilot behaviour (IntelligentPooling pilot, Oralytics "did we learn"), and a randomized behavioural outcome (DIAMANTE, MyBehavior's 17-person trial). Only the last grade shows users actually did more.
- MyBehavior is the closest analogue to the companion app in *content* (suggestions derived from the user's own behaviour rather than generic advice) and it is also the smallest and shortest trial.

### Gaps
- I found no study of a contextual bandit for push timing run on a single user with no pooled data and no prior, with a reported outcome.
- I did not find a published primary-outcome paper for the Oralytics trial (effect of the RL-delivered prompts on brushing) in the sources I read.
- A "PEARL" trial was named in my search plan but no source for it was found; nothing is reported here about it.

---

## 6. Industry evidence: Duolingo, Pinterest, Twitter, LinkedIn, Meta, email send-time optimization

### Takeaway
Industry systems operate at millions of sends and report small percentage gains from bandit template selection and from *reducing* volume; the one long-horizon experiment found (Meta) reports that cutting notifications lowered visits at first and raised them after about a year, meaning short experiments pointed the wrong way. Rigorous public evidence on send-time optimization is thin and mostly vendor-reported.

### Cited Findings

**Duolingo — "A Sleeping, Recovering Bandit Algorithm for Optimizing Recurring Notifications" (Yancey & Settles, KDD 2020)**
- Problem: choose which practice-reminder template to send each learner each day. "Sleeping" arms: not every template is eligible for every user (e.g. streak-specific or Monday-only templates), so each template is scored only against others sent to the same kind of learner. "Recovering" arms: a template's effect drops after a user has recently seen it and recovers with time, so recently used templates are demoted. [blog] — [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/); paper: [Yancey & Settles 2020](https://research.duolingo.com/papers/yancey.kdd20.pdf)
- Novelty: the team hypothesised that fresh notifications are especially persuasive and that the effect wears off, and "Their data confirmed it"; repeat spacing uses the same forgetting-curve form Duolingo uses for vocabulary. [blog] — [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/)
- Scale: trained on about 200 million practice reminders sent over roughly 34–35 days; reward is whether the learner completes a lesson (the public dataset records conversion within 2 hours). [blog / research page] — [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/); [Duolingo Research](https://research.duolingo.com/)
- Reported online results: DAU +0.5%, lessons completed +0.4%, new-user retention +2%. [secondary: figures from a third-party summary; I could not parse the paper PDF to confirm] — [Eugene Yan, "Push Notifications: What to Push, What Not to Push, and How Often"](https://eugeneyan.com/writing/push/)
- Timing: the Duolingo work optimizes *which* template, not *when*; the blog says nothing about send-time optimization. [blog] — [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/)

**Pinterest — "Notification Volume Control and Optimization System at Pinterest" (Zhao et al., KDD 2018)**
- Method: randomly assigned weekly volumes to collect unbiased data; modelled unsubscribe probability and long-term activity after unsubscribing (measured with a four-week delay); set per-user budgets by incremental value. Reported results: notification volume down 6–24%, click-through rate up 11–31%, site engagement up 1–3%; core users got fewer messages and marginal users more. [secondary: third-party summary of the paper] — [Eugene Yan](https://eugeneyan.com/writing/push/); paper: [Zhao et al. 2018](https://dl.acm.org/doi/10.1145/3219819.3219906)

**Twitter — push decision by modelling the future (O'Brien et al. 2022) and hybrid push caps**
- Framing: "a myopic system will always choose to send a notification since negative effects occur in the future"; too many or irrelevant notifications may lead users to disable them. The paper uses model-based RL to optimize long-term value, tracking daily active users, "reachability" (users with push enabled) and open rate. [full text, abstract and intro] — [O'Brien et al. 2022, arXiv 2202.08812](https://arxiv.org/abs/2202.08812)
- Personalized push caps with long-term labels collected over two months: DAU +0.62% on iOS and +0.39% on Android with insignificant reachability loss. [secondary: third-party summary of Twitter's engineering blog] — [Eugene Yan](https://eugeneyan.com/writing/push/)

**LinkedIn**
- One-week experiment dropping about half of emails at random: 2.6% fewer page views in the reduced group (by area: homepage −1.4%, jobs −4%, profile −4.5%, people-you-may-know −4.5%, search −4%); the send-all group had 45% more negative responses (unsubscribes and spam reports). The post notes an unsubscribe permanently removes the ability to send that email type. It reports no long-horizon follow-up. [primary company blog] — [LinkedIn Engineering, "Less Is More: Optimizing Email Volume, Part 1"](https://www.linkedin.com/blog/engineering/archive/less-is-more-optimizing-email-volume-part-1)
- LinkedIn stated in 2015 that it had cut email by about 40% and halved complaints. [secondary: search excerpt] — [LinkedIn blog 2015](https://blog.linkedin.com/2015/07/27/less-email-from-linkedin)
- "Near Real-Time Optimization of Activity-Based Notifications" (Gao et al., KDD 2018) describes a response-prediction and filtering system; the summary I read reports no engagement percentages. [secondary] — [Eugene Yan](https://eugeneyan.com/writing/push/); paper: [Gao et al. 2018](https://dl.acm.org/doi/abs/10.1145/3219819.3219880)

**Meta / Facebook — "Notifications: why less is more"**
- Facebook ran an experiment sending only the most relevant notifications and kept it running for about a year. Short term: a loss of visits. Long term: the loss "gradually recovered over time" and "after an extended period, it had fully recovered and even turned out to be a gain"; satisfaction also rose. Stated lesson: "Experiments in the very long-run can show different results than in the short run." No numeric effect sizes are given in the excerpts. [secondary: the original Medium post returned an access error; read via a page quoting it] — [Analytics at Meta (Medium)](https://medium.com/@AnalyticsAtMeta/notifications-why-less-is-more-how-facebook-has-been-increasing-both-user-satisfaction-and-app-9463f7325e7d); quoted at [exde601e blog](https://exde601e.blogspot.com/2023/01/notifications-why-less-is-more.html)

**Email send-time optimization**
- I found no peer-reviewed Google or Microsoft paper reporting a randomized send-time experiment. Available figures are vendor claims: Iterable reports median US lifts of 82% in opens and 73% in clicks from its send-time feature while stating the feature "doesn't guarantee higher engagement", and notes that Apple Mail Privacy Protection affects roughly 55–60% of opens so clicks are the more reliable signal. [vendor source] — [Iterable](https://iterable.com/blog/what-is-send-time-optimization/)
- Another vendor case study reports mixed results: one method raised opens about 10% and clicks 35%; another produced a lower open rate than control. [vendor source] — [Yespo](https://yespo.io/blog/personal-email-sending-time)

### Inferences
- Short-term and long-term effects of volume point in opposite directions in the two sources that measured both sides: LinkedIn's one-week test shows fewer messages cost about 2.6% of page views immediately while cutting negative responses by roughly a third; Meta's year-long test reports the immediate visit loss reversing into a gain.
- Industry gains from bandits are fractions of a percent to a few percent and are detectable only at very large scale; the algorithms depend on population data (Duolingo's template scores are learned across all users, not per user).
- Industry optimizes return visits to an app; the companion app's outcome is a person acting on their own intention. The objective differs, so effect sizes should not be carried across; the structural findings (novelty decay, recovery with spacing, delayed cost of volume) are the transferable part.

### Gaps
- Duolingo, Pinterest and Twitter figures above are from a secondary summary; I could not confirm them in the primary PDFs.
- Meta's post gives no magnitudes in the text I could access.
- No credible, non-vendor randomized evidence on per-user send-time optimization for email or push was found.
- I did not find Spotify or Slack publications on notification timing with measured effects.

---

## 7. What is defensible for one user with a handful of data points per week

### Takeaway
The sources do not test single-user, few-events-per-week learning directly; what they do say is that per-user learning at 2–5 decisions per day was already judged too data-poor to stand alone, that informative priors and very low-dimensional models are the standard remedy, and that send probabilities should be bounded so the system neither locks in on early responses nor stops exploring.

### Cited Findings
- Single-user data alone is considered insufficient even at five decisions a day: in-situ noise "can cripple the ability of an algorithm to learn when given access to only a single user's data". [full text] — [Tomkins et al.](https://arxiv.org/abs/2002.09971)
- 140 decisions per person was treated as too few for per-person models, leading to a single shared model; in simulation, pooling and not pooling performed almost identically (69.724 vs 69.375). [full text] — [Trella et al. 2024](https://arxiv.org/abs/2409.02069)
- Priors from earlier population data are used specifically to protect against early noise: the informative prior is there "to speed up the learning in the early phase of the study as well as to reduce the variance and diminish the impact of noisy observation". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Model size is deliberately small: a low-dimensional linear model of the treatment *difference* "to trade off the bias and variance to accelerate learning"; the IntelligentPooling pilot used only two person-specific random effects because it "only includes a small number of users". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539); [Tomkins et al.](https://arxiv.org/abs/2002.09971)
- Guarding against lock-in: HeartSteps V2 clips the send probability so both sending and not sending keep a minimum probability (0.1 and 0.2), so the algorithm "will continue to explore and learn, instead of locking itself into a particular policy". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Cost of the exploration floor: with a floor in place an unresponsive pilot participant still received suggestions at probability 0.2–0.4, which the authors called "likely too much". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Slow adaptation with little data: the algorithm took about six days of declining responsivity (at up to five decisions a day) before reducing a participant's send probability. [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Personalization emerges gradually: the divergence between personalized and pooled send probabilities grew with accumulated user data (r = .56, p < .1, ten users). [full text] — [Tomkins et al.](https://arxiv.org/abs/2002.09971)
- Simple rules remain the framework's baseline: a JITAI decision rule is any systematic link between tailoring variables and options, and the FOCUS example is a plain if-then rule (no follow-up if the prompt was ignored). [full text] — [Nahum-Shani et al. 2018](https://pmc.ncbi.nlm.nih.gov/articles/PMC5364076)
- Fixed-budget randomization is an accepted alternative to learning: spreading a capped number of prompts uniformly over eligible moments both limits burden and yields data for later analysis. [abstract] — [Liao et al. 2018](https://doi.org/10.1145/3287057)
- Non-stationarity is the norm: effects decline with dose and time (HeartSteps V1), and a plain Thompson-sampling bandit is described as "sensitive to model misspecification/non-stationarity and is greedy to maximize the immediate rewards". [full text] — [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- A context-free bandit (EXP3) over a user's own behaviour candidates was enough to produce a measurable difference in a 17-person, 3-week trial (walking P = .05). [full text] — [Rabbi et al. 2015](https://doi.org/10.2196/mhealth.4160)

### Inferences
- Reading these together: at 3–10 events a week, the approaches the literature treats as workable are the ones with the fewest free parameters — fixed rules with a volume cap; a per-category success rate with a strong prior (the Beta-Bernoulli case of the Thompson sampling these papers use); and bounded send probabilities. Anything with context features is, by these authors' own reasoning about 140–450 decisions per user, under-determined for a single user.
- The app has no population to pool over, so the role played by pooling in IntelligentPooling and Oralytics would have to be played entirely by the prior. The sources support using a prior; they do not tell us what a good prior is for self-set intention reminders.
- Because effects decay with repetition, a stationary per-category estimate will lag; the sources that address this add an explicit recent-dose term (HeartSteps V2, Oralytics) or a recency penalty (Duolingo) rather than relying on the bandit to notice.
- An exploration floor trades off against the "stay silent" goal: the HeartSteps pilot shows the floor itself can over-send to a non-responder.

### Gaps
- No source gives a minimum number of observations for a Beta-Bernoulli or any other per-user bandit to outperform a fixed rule; the "minimum data" statements above are design judgements by the authors, not tested thresholds.
- No source evaluated exploration rates (for example a fixed epsilon) at single-user, few-per-week volume.
- I found no evidence on whether explicit feedback buttons (do it / later / not interested) are a better learning signal than passive outcomes; all the trials above learn from sensed or logged behaviour.

---

## 8. Self-reported timing preferences versus learned behaviour

### Takeaway
I found no study that directly compares a user's stated preferred time with the time they actually respond or act. Adjacent evidence is mixed: models learned from behaviour can beat random timing on response to the message, but sophisticated timing did not beat a simple fixed evening window in one trial, and better message response did not always change the target behaviour.

### Cited Findings
- Behaviour-learned timing vs random timing: ML receptivity models gave up to a 40% improvement in receptivity over random-time delivery, and the per-user adaptive model's receptivity rose over the study. [abstract] — [Mishra et al. 2021](https://arxiv.org/abs/2011.08302)
- Behaviour-sensed timing vs a fixed window: 77 participants; "No meaningful differences were indicated between the intelligent and non-intelligent groups"; "adaptive tailoring of notification timing does not always enhance their use". The fixed window (17:00–20:00) was chosen by the researchers from prior research on non-working hours, not by participants. [abstract + full-text methods] — [Morrison et al. 2017](https://journals.plos.org/plosone/article?id=10.1371/journal.pone.0169162)
- Population usage data as the basis for timing: Drink Less sent at 8 PM because observed app use peaked then; with that timing notifications tripled opening within the hour. Other times were not tested. [full text] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Time-of-day and day-of-week differences in effect were suggestive but not significant in JOOL (weekend 8.7% vs weekday 2.5%, P = .18). [full text] — [Bidargaddi et al. 2018](https://www.ncbi.nlm.nih.gov/pmc/articles/PMC6293241/)
- Context correlates of in-the-moment receptivity (day/time, phone interaction, activity, location) were identified from behaviour in 189 users. [abstract] — [Künzler et al. 2019](https://doi.org/10.1145/3369805)
- User-selected times in practice: HeartSteps V1 and V2 both let users choose their five daily decision times, then learned or randomized *within* those times; neither reports a comparison of chosen times against other times. [abstract / full text] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/); [Liao et al. 2020](https://arxiv.org/abs/1909.03539)
- Stated delivery preference vs behaviour: in a one-week study of 35 Android users, response time did not differ significantly across users' stated delivery preferences for the same notifications. [secondary: search excerpt] — [Alert Now or Never, IMWUT 2021](https://dl.acm.org/doi/10.1145/3478868)
- An earlier study reported that self-reports about notification timing "did not always match" the behavioural results. [secondary: search excerpt] — [Fischer et al., MobileHCI 2011](https://interruptions.net/literature/Fischer-MobileHCI11.pdf)
- Timing can change response to the message without changing the behaviour: context-triggered notifications in a weight-maintenance app affected click response time, click rate and completion rate, with no difference in overall self-monitoring frequency. [secondary: search excerpt] — [CHI 2024](https://dl.acm.org/doi/10.1145/3613904.3641993)

### Inferences
- The available evidence compares *learned/sensed* timing against *random* or *researcher-fixed* timing, not against *user-stated* timing, so it cannot say whether stated preferences are good or bad predictors.
- The common research design (HeartSteps) uses stated times as the outer constraint and learns inside it. That design is used, not validated, in the sources I read.

### Gaps
- Direct evidence on whether stated preferred times predict actual response times: not found. This is a real gap in what I could locate, not a negative finding.
- The two stated-preference sources above are search excerpts I did not verify in full text.
- No evidence was found on timing preferences for reminders of self-set intentions.

---

## Cross-cutting: systematic reviews and meta-analyses of JITAI effectiveness

### Takeaway
Pooled JITAI effects in meta-analyses are moderate to large, but the reviews themselves flag small, under-powered and heterogeneous studies; the pooled numbers describe whole intervention packages against controls, not the marginal effect of a single well-timed prompt.

### Cited Findings
- Wang & Miller 2020: 33 studies; JITAI vs waitlist control Hedges's g = 1.65 (k = 9); vs non-JITAI treatment g = 0.89 (k = 21); within-group change g = 0.79 (k = 13). Greater efficacy was associated with tailoring to both past behaviour patterns and current need state, and with combining a human and an algorithm in deciding feedback. [abstract] — [Wang & Miller 2020, Health Communication](https://doi.org/10.1080/10410236.2019.1652388)
- Hardeman et al. 2019: 19 papers on 14 physical-activity JITAIs, typically three to four weeks long; "mixed evidence for intervention effects on behaviour, but no study was sufficiently powered to detect any effects"; no evidence on sustained engagement. [abstract] — [Hardeman et al. 2019, IJBNPA](https://doi.org/10.1186/s12966-019-0792-7)
- Xu & Smit 2023 report a medium pooled effect of JITAIs on objectively measured behaviour, g = 0.77 (95% CI 0.32–1.22). [secondary: search excerpt] — [Xu & Smit 2023, Digital Health](https://doi.org/10.1177/20552076231183543)
- Yang et al. 2026 (search to June 2026): 37 physical-activity JITAI studies, 6,939 participants; supportive RCTs reported +371 to +1,327 steps/day; exploratory meta-analysis showed substantial heterogeneity (I² = 85%). [abstract] — [Yang et al. 2026, Frontiers in Public Health](https://doi.org/10.3389/fpubh.2026.1934024)
- Xi et al. 2026: 11 RCTs (3 EMIs, 8 JITAIs) for sleep; insomnia severity SMD = −0.87 (95% CI −1.32 to −0.42); sleep efficiency SMD = 0.71 (0.17–1.25); no significant difference between JITAIs and non-adaptive momentary interventions on any outcome. [abstract] — [Xi et al. 2026, Sleep Medicine](https://doi.org/10.1016/j.sleep.2026.109349)

### Inferences
- The meta-analytic effect sizes (g ≈ 0.8–1.6) and the MRT proximal effects (often a few percent, sometimes zero) measure different things. The MRT figures are the relevant ones for predicting what one additional nudge does.
- The 2026 sleep review's finding of no JITAI-vs-EMI difference is one data point against assuming that adaptivity itself adds effect.

### Gaps
- No review I found pools decay or habituation estimates across MRTs.
- No review addresses prompts about self-generated goals as a distinct category.
