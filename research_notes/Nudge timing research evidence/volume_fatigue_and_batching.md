# Notification volume, fatigue, batching and digests

Notes compiled 14 November 2026. Evidence grades used below: **[RCT]** randomised experiment, **[MRT]** micro-randomised trial, **[Obs]** observational or log study, **[Survey]** self-report survey, **[Meta]** meta-analysis, **[Vendor]** vendor report or company blog (lower quality, self-interested, usually no methods), **[Anecdote]** single-person account. Where I read the full paper I say so; otherwise figures come from abstracts or secondary summaries and are flagged.

## 1. Fitz et al. 2019 (batching) and replications

### Takeaway
One two-week RCT (N = 237, India, Android) found that batching all notifications three times a day gave medium-sized improvements in attention, stress, mood, productivity and sense of control, while hourly batching did almost nothing and removing notifications entirely raised anxiety and fear of missing out. I found no direct replication of the batching arms from 2020 to 2026; the one preregistered follow-up on the "no notifications" arm (Dekker et al., 2024) reproduced the fear-of-missing-out cost but found no behavioural benefit.

### Cited Findings
- Design (full paper read): N = 237 completers recruited on MTurk in March 2017, all smartphone owners in India (mean age 30.3, 19% female, 85% employed). One baseline week, then two experimental weeks, between-subjects, four arms: notifications as usual (control), batched hourly on the hour, batched three times a day at 9am, 3pm and 9pm, or no notifications at all. A custom Android app enforced the schedule and participants could not change it, but they could open any app to check manually. Outcomes were end-of-day diary items plus weekly multi-item scales, and phone unlocks were logged passively. [RCT] — [Fitz et al. 2019, Computers in Human Behavior 101:84–94](https://doi.org/10.1016/j.chb.2019.07.016); full text read from [author-hosted PDF](https://static1.squarespace.com/static/57a40c19414fb54f51f8095f/t/614a55faa7b89e25f4e48ad1/1632261627146/2019+Fitz+Batching.pdf)
- Three-times-a-day batch versus control (Cohen's d from the paper's Table 2, tests not adjusted for multiple comparisons): inattention scale d = −0.65 (p < .01); concentration d = 0.54; perceived productivity d = 0.58 (scale) and 0.52 (daily item); stress d = −0.56 (scale) and −0.40 (daily item); mood d = 0.49 (p < .05); sense of control over the phone d = 0.58; felt interrupted by notifications d = −0.48; phone unlocks 37.2 per day versus 57.3 in control, d = −0.60. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- Cost of the three-times-a-day batch: fear of missing out rose. "Felt I was missing out on important notifications" d = 0.68 (p < .01) and phone-related FoMO scale d = 0.43 (p < .05). Anxiety did not differ meaningfully from control (d = 0.17 to 0.19). — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- Hourly batch versus control: the paper states it "did not differ from the control, except for a single effect on the daily measure of feeling interrupted by notifications" (d = −0.43). Unlocks 50.0 versus 57.3 (d = −0.15, not significant). — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- No-notifications arm versus control: felt less interrupted (d = −0.74) but no gain in control over the phone (d = 0.15), inattention (d = 0.10) or concentration; higher anxiety (scale d = 0.56, p < .01), higher phone FoMO (d = 0.60) and higher sense of missing important notifications (d = 0.53). Mediation analysis attributed the anxiety increase to phone-related FoMO. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- No effects in any arm on depression, mindfulness, positive affect, social connectedness, phone overuse, phone addiction, nomophobia or felt social pressure to respond (all omnibus p > .25). Participants did not notice the benefits when asked afterwards. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- Participants wanted to receive about half as many notifications as they believed they received, and greatly underestimated how many they actually received (paper's Fig. 5; exact counts not given in the text). — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- Limitations stated by the authors: two-week treatment only; blanket batching that ignores which notifications matter; restrictive, non-overridable control may itself have raised anxiety, and the authors stress "the importance of granting personal control over phone settings"; India-only sample. Completer analysis: 96 people dropped after onboarding and a further 108 (31%) did not finish, though dropout did not differ by arm (p = .67). — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- Reporting inconsistencies in the published paper: the abstract says the batched group reported "lower productivity", and the body text says "greater stress (d = 0.56)"; Table 2 shows the opposite direction for both (productivity up, stress down), which is also what the authors' own summaries say. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016); [authors' summary](https://communities.springernature.com/posts/batching-notifications-can-improve-attention-well-being-and-productivity)
- Follow-up on removing notifications (March 2024 online, Media Psychology vol. 28): preregistered RCT, N = 205, one week of disabled notifications with logged phone data and daily diaries. No effect on logged checking frequency or screen time, regardless of trait FoMO. Perceived checking-habit strength fell, but there were no effects on perceived control, overuse, vigilance, productivity or distraction, and FoMO increased. Figures are from the abstract. [RCT] — [Dekker, Baumgartner, Sumter & Ohme, "Beyond the Buzz"](https://doi.org/10.1080/15213269.2024.2334025)
- Related observational result (2022): among 138 iPhone users, phone checks averaged 52.9 per day with sound and vibration on versus 98.2 per day on silent; high-FoMO users checked about 120 times a day on silent. Cross-sectional screen-time data, so not causal; figures from the university press release. [Obs] — [Liao & Sundar 2022, Penn State summary](https://www.psu.edu/news/institute-computational-and-data-sciences/story/turning-phones-silent-may-increase-phone-checking)

### Inferences
- The benefit in Fitz et al. came from a few predictable delivery times, not from "less often" in general: the hourly arm also cut interruptions but produced no well-being gain.
- Across Fitz (2019), Pielot & Rello (2017) and Dekker (2024), withholding notifications entirely consistently raises worry about missing things. This is the best-replicated finding in this group, and it applies to a digest that hides items the person would have wanted sooner.
- Fitz et al. batched every notification on the phone (dozens per day, mostly messages). A single low-volume app batching its own few reminders is a different intervention, so the effect sizes do not transfer.

### Gaps
- No direct replication of the three-times-a-day or hourly batching arms was found for 2020–2026. The positive batching result rests on one study.
- No study was found testing batch intervals between three per day and weekly for well-being outcomes.
- Per-arm sample sizes for Fitz et al. are not stated in the text I extracted.

## 2. Pielot & Rello 2017 and email batching (Mark et al.; Kushlev & Dunn)

### Takeaway
A day without notifications made 30 volunteers feel less distracted and more productive, but also less responsive, more worried and less connected, with no change in stress. Limiting email checks to about three times a day lowered daily stress in one crossover experiment, while an observational workplace study found a productivity association for batching but no stress benefit.

### Cited Findings
- Pielot & Rello (study run 2015, published MobileHCI 2017; full preprint read): 30 volunteers disabled notification alerts on all devices for 24 hours, compared within-subject against a baseline day; 5-point agreement items, Wilcoxon tests, Cliff's delta. [RCT, within-subject, small] — [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314)
- Without notifications: "I felt distracted" mean 1.9 versus 2.6 (p = 0.020, δ = −0.32); "I felt productive" 3.9 versus 3.4 (p = 0.011); "I was as responsive as usual" 3.0 versus 4.0 (p = 0.001, δ = −0.54); "I felt worried about missing notifications" 2.9 versus 1.8 (p = 0.001, δ = 0.42); "I felt connected with my social group" 3.1 versus 3.9 (p = 0.002, δ = −0.34); "I felt stressed" 2.0 versus 2.0 (not significant). — [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314)
- 12 of 30 said they checked their devices more often than usual on the day without notifications; 9 of 30 reported anxiety about missing important or urgent information; 8 reported actually missing something important. — [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314)
- 22 of 30 (73.3%) said they intended to change how they manage notifications (selective disabling or Do Not Disturb). Contacted two years later (April 2017), 13 of those 22 (59.1%) had followed through. — [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314)
- Earlier deprivation studies as summarised in that paper: Iqbal & Horvitz (2010) had 20 information workers turn off email notifications for one week; most checked email less opportunistically but none kept notifications disabled afterwards. Kushlev et al. (2016) ran one-week conditions with 221 participants. — [Pielot & Rello 2017, related work](https://arxiv.org/abs/1612.02314)
- Kushlev & Dunn (2015, Computers in Human Behavior 43:220–228): 124 adults, two-week crossover; one week limited to checking email three times a day, one week unlimited. Daily stress was significantly lower in the limited week, and lower stress predicted better well-being. Compliance was partial: about 4.5 checks a day in the limited week versus 12.5 in the unlimited week. Participants did not feel more productive in the unlimited week. Effect sizes not retrieved; compliance figures come from press coverage. [RCT, crossover] — [Kushlev & Dunn 2015 PDF](https://www.interruptions.net/literature/Kushlev-ComputHumBehav15.pdf); [New Republic summary](https://newrepublic.com/article/120491/youll-be-less-stressed-if-you-check-your-email-less-frequently)
- Mark, Iqbal, Czerwinski et al. (CHI 2016): 40 information workers tracked for 12 workdays with computer logs, heart-rate sensors and daily surveys. More daily time on email was associated with lower perceived productivity and higher measured stress. People who checked email by self-interruption, and people who batched, rated their productivity higher for a given email duration than those relying on notifications, but batching was not associated with lower stress. Summary-level reading only. [Obs] — [Mark et al. 2016](https://www.microsoft.com/en-us/research/wp-content/uploads/2016/06/Email20Duration20Camera20Ready20submission3-1.pdf)

### Inferences
- The experimental evidence (Kushlev & Dunn) and the observational evidence (Mark et al.) disagree on whether batching email lowers stress; they agree that it does not hurt perceived productivity.
- People who try life with fewer alerts often choose selective disabling afterwards (13 of 30 in the two-year follow-up), which suggests per-category controls are used when the person has experienced the difference.

### Gaps
- Pielot & Rello lasted one day with 30 self-selected volunteers; no long-duration removal study with objective outcomes exists other than Dekker et al. (one week).
- I did not verify the secondary claim that the study was shortened to one day because nobody would volunteer for longer.

## 3. How many notifications people receive, and what share is acted on

### Takeaway
Logged adult volumes sat around 56–63 per day in 2013–2016 studies, and a 2023 study of US teens logged a median of 237 per day. Messaging notifications are opened about two-thirds of the time; everything else is opened roughly 15–27% of the time, and web push interaction is under 1%.

### Cited Findings
- Pielot et al. (2014, data from 2013): 15 participants received an average of 63 notifications per day and mostly triaged them within minutes (as cited in the 2018 paper). [Obs] — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- Pielot, Vradi & Park (MobileHCI 2018; full paper read): 794,525 notifications from 278 Android users in Spain, logged for about four weeks in summer 2016, sample matched to national age and gender distribution. Median 56 notifications per day; the authors say this "does not indicate that the number of notifications has increased". Daily medians by type included about 18 individual-message, 7 group-message, 2 email and 2 non-social app notifications. [Obs] — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- Share that led to opening the app (same study): individual messaging 63.67%, group messaging 65.81%, social 26.55%, non-social apps 16.19%, email 15.47%. 20–35% of notifications arrived while the phone was already unlocked. — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- That study excluded alarm-clock and calendar-reminder notifications "as we found that users typically do not attend them by opening the app", so reminder-type notifications have no conversion rate in this dataset. — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- Earlier figures cited there: Sahami Shirazi et al. found that when a notification was clicked, it happened within a median of 30 seconds; Mehrotra et al. found 20 participants clicked 60% of notifications within 10 minutes. — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- Common Sense Media and University of Michigan (September 2023): 203 US 11–17-year-olds on Android, tracked for one week; median 237 notifications per day, about 25% during school hours and 5% at night; maximum over 4,500 delivered in a day. Teens only, so not comparable with the adult figures. [Obs] — [WebMD report on "Constant Companion"](https://www.webmd.com/teens/news/20230927/cm/teens-get-hundreds-of-smartphone-notifications-daily-study)
- Google Chrome (10 October 2025): "less than 1% of all notifications receive any interaction from users". This is website push in the browser, not app push, and is Google's own figure. [Vendor] — [Chrome blog](https://blog.google/chromium/automatic-notification-permission/)
- Fitz et al. baseline survey (India, 2017): 68% said they receive notifications at least "a few times an hour". [Survey] — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)

### Inferences
- A companion app sending one or two messages a day would be roughly 2–4% of a typical adult's 56–63 daily notifications, so its own volume is not what drives overall overload; what matters is whether its messages land in the roughly 16% open-rate class of generic app notifications or closer to the roughly 64% class of personal messages.
- Reminder and productivity apps do not have their own category in the large log studies. They fall under "non-social" (16% opened) or are excluded.

### Gaps
- No logged adult notification count more recent than 2016 data was found; the 2023 figure is teens only.
- No published open or action rate specifically for to-do, reminder or productivity app notifications was found.

## 4. Habituation and alert fatigue

### Takeaway
Response to repeated, similar alerts declines within days to weeks in security warnings, clinical alerts and one physical-activity trial, and varying the alert's appearance or resting it slows the decline. The decline is not universal: two app-engagement micro-randomised trials found no significant weakening over one to three months.

### Cited Findings
- Vance, Jenkins, Anderson, Bjornn & Kirwan (MIS Quarterly 42(2), June 2018). Lab study: fMRI and eye tracking across a five-day work week showed "a general decline in attention over time" to repeated warnings, with attention recovering "at least partially between workdays without exposure". Field study: participants installed three apps a day for 15 days over three weeks from a purpose-built Android store and saw permission warnings each time. By the end, users shown identical warnings adhered 55% of the time; users shown polymorphic warnings (appearance changes each time) adhered 76% of the time. [RCT] — [MISQ abstract](https://misq.org/tuning-out-security-warnings-a-longitudinal-examination-of-habituation-through-fmri-eye-tracking-and-field-experiments.html); [BYU summary with the 55%/76% figures](https://marriott.byu.edu/stories/faculty-research/paying-attention-to-notifications-1412)
- Anderson, Vance et al. (Journal of MIS, 2016): fMRI study, n = 25; polymorphic warnings were significantly more resistant to habituation than conventional ones in attention-related brain regions. [RCT, lab] — [JMIS listing](https://jmis-web.org/articles/1304)
- Authors' design advice from the 2018 study: "Even using a few variations can have a substantial effect over time", and designers "need to be as judicious as possible with the number of warnings they present". — [BYU summary](https://marriott.byu.edu/stories/faculty-research/paying-attention-to-notifications-1412)
- Clinical decision support, Ancker et al. (BMC Medical Informatics and Decision Making, 2017): retrospective cohort of 112 primary care clinicians, 2010–2013. The likelihood of accepting a reminder fell by 30% for each additional reminder received per patient encounter, and clinicians accepted fewer alerts as they received more, "particularly more repeated alerts". There was no evidence of desensitisation over time to a newly deployed alert, and no effect of overall workload. A quarter of drug alerts and a third of clinical reminders were repeats for the same patient within a year. [Obs] — [Ancker et al. 2017](https://pmc.ncbi.nlm.nih.gov/articles/PMC5387195)
- HeartSteps (Klasnja et al., Annals of Behavioral Medicine, 2019): six-week MRT, 44 adults, up to five randomised activity suggestions a day. Averaged over the study, a suggestion raised the next-30-minute step count by 14% (35 steps, p = .06). At the start of the study the effect was 66% (167 steps, p < .01) and it "diminished over time". Walking suggestions: 24% averaged (59 steps, p = .02), 107% at the start (271 steps, p < .01), decreasing over time. Antisedentary suggestions had no detectable effect. [MRT] — [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Drink Less (Bell et al., JMIR mHealth and uHealth, 2023): MRT with 350 new users, randomised each day to receive or not receive an evening notification. Receiving one raised the probability of opening the app in the next hour 3.5-fold (95% CI 2.91–4.25), and the effect "did not change significantly over time" across the roughly 30-day window. Time to disengagement did not differ between notification policies. [MRT] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342)
- Workplace well-being app (Bidargaddi et al., JMIR mHealth and uHealth, 2018): MRT, 1,255 users, 89 days, at most one randomised push per day. A tailored push raised the chance of self-monitoring in the next 24 hours by 3.9% (risk ratio 1.039, 95% CI 1.01–1.08). The effect attenuated over the study but not significantly (p = .84). [MRT] — [Bidargaddi et al. 2018](https://doi.org/10.2196/10123)
- Duolingo (KDD 2020 paper and company blog): analysis of about 200 million practice reminders showed new reminder templates work better at first and lose effect as novelty wears off. A bandit that accounts for how recently each template was shown ("recovering" arms) raised daily active users by 0.5% and new-user retention by 2% over the previous system. [Vendor, peer-reviewed venue] — [Yancey & Settles 2020](https://research.duolingo.com/papers/yancey.kdd20.pdf); [Duolingo blog](https://blog.duolingo.com/hi-its-duo-the-ai-behind-the-meme/)

### Inferences
- Timescale: where decline was measured it showed up within one to six weeks (security warnings over 15 days, HeartSteps over 42 days). The two null results came from apps sending at most one notification per day, while HeartSteps sent up to five per day; this pattern is consistent with habituation depending on dose, but the studies differ in too many ways to conclude that.
- Three things have experimental support for slowing or reversing decline: changing how the alert looks or reads (Vance 2018; Duolingo), gaps without exposure (Vance 2018 partial recovery between days; Duolingo's recency model), and cutting repeats of the same alert about the same item (Ancker 2017, observational).
- The clinical finding that acceptance falls with each extra alert in the same encounter argues against stacking several nudges about different items into the same moment.

### Gaps
- Sample sizes for the two Vance 2018 experiments and the starting adherence rates were not retrieved.
- No study measures habituation to reminders about a person's own self-set tasks over months.
- How long a rest period is needed for full recovery is not quantified in any source found.

## 5. Industry data on frequency versus opt-out, uninstall and engagement

### Takeaway
Platform experiments from Meta, LinkedIn, Instagram and an unnamed large social network all report that sending fewer, better-selected notifications cut complaints or held engagement, and Meta reports a long-run gain; none publish thresholds. The only published frequency thresholds are self-report surveys (Localytics) in which a sizeable minority say two to five pushes a week from one app would make them turn notifications off.

### Cited Findings
- Meta / Facebook (Analytics at Meta post, reposted January 2023): a test sent "considerably fewer" notifications, keeping only those predicted to get a top (5 of 5) rating in a user survey. Visits dropped at first; the test was extended for about a year or more, the drop "gradually recovered", and it eventually "turned out to be a gain". Satisfaction also improved. No percentages are published, only a chart. [Vendor] — [Analytics at Meta, "Notifications: why less is more"](https://medium.com/@AnalyticsAtMeta/notifications-why-less-is-more-how-facebook-has-been-increasing-both-user-satisfaction-and-app-9463f7325e7d); [repost read for these notes](https://exde601e.blogspot.com/2023/01/notifications-why-less-is-more.html)
- Context for the Meta result (August 2026): a workshop of 26 experts from 15 platforms and 4 universities concluded that sign reversals between short-run and long-run experiment effects are rare overall; the abstract does not mention notifications. — [Sigerson et al. 2026, "Evaluating for the long term"](https://arxiv.org/abs/2608.08043)
- LinkedIn email (2015 and 2016 engineering posts): A/B testing let LinkedIn send 40% less email and halve email complaints. In a one-week test where roughly half of generated emails were dropped at random, the random-drop group had 2.6% fewer page views, while the send-everything group had 45% more negative responses (unsubscribes or spam reports). LinkedIn concluded that naive cuts hurt engagement and moved to per-email optimisation. [Vendor] — [LinkedIn 2015](https://engineering.linkedin.com/blog/2015/10/how-experimentation-helped-linkedin-improve-email-communication); [LinkedIn 2016, "Less is more"](https://www.linkedin.com/blog/engineering/archive/less-is-more-optimizing-email-volume-part-1)
- LinkedIn's central notification gateway (2018) was built because "repetitive, excessive and low-quality notifications" lead members to complain, disable notifications and use the app less. No figures given. [Vendor] — [LinkedIn, Air Traffic Controller](https://engineering.linkedin.com/blog/2018/03/air-traffic-controller--member-first-notifications-at-linkedin)
- Instagram (October 2022): replacing a click-prediction model with an uplift model "reduced the sending volume substantially" with no decline in engagement. No figures given. [Vendor] — [Meta Engineering](https://engineering.fb.com/2022/10/31/ml-applications/instagram-notification-management-machine-learning/)
- Unnamed "major social network" (O'Brien et al., February 2022): a reinforcement-learning sender that models effects several days ahead sent fewer notifications than the heuristic baseline in an A/B test, kept engagement level and got a higher open rate. The abstract gives no magnitudes. [Vendor, preprint] — [O'Brien et al. 2022](https://arxiv.org/abs/2202.08812)
- Pinterest (KDD 2018 paper; 2020 engineering post): each user gets a weekly notification budget set at the volume that maximises predicted incremental engagement for that user. No public numbers on the budgets. [Vendor, peer-reviewed venue] — [Pinterest Engineering](https://medium.com/pinterest-engineering/user-state-based-notification-volume-optimization-7764118f73ff); [Zhao et al. KDD 2018](https://dblp.dagstuhl.de/rec/conf/kdd/ZhaoNOE18.html)
- Academic field experiment, Wohllebe et al. (Innovative Marketing 17(2), 2021): 17,500 users of a German retailer's app, five push frequencies, seven weeks, generic non-personalised pushes. Uninstalls rose and the direct open rate fell as frequency rose. Per-arm frequencies and rates not retrieved. [RCT] — [Wohllebe et al. 2021](https://businessperspectives.org/images/pdf/applications/publishing/templates/article/assets/15070/IM_2021_02_Wohllebe.pdf)
- Localytics surveys of US smartphone users. November 2017 wave (n = 1,000): about 22% said two to five pushes a week from an app would make them stop using it (31% in the October 2015 wave), and about 31% named six to ten a week. One summary of the same wave reports 37% would disable notifications at two to five a week, 33% at six to ten, and about 10% at one a week. The 2015 wave, via WAN-IFRA: 46% would disable push at two to five a week, and 32% would stop using the app at six to ten. Sources disagree on which wave and question each number belongs to. [Vendor, Survey, hypothetical self-report] — [MarketingProfs summary](https://www.marketingprofs.com/charts/2018/33486/how-many-mobile-push-notifications-are-too-many); [WAN-IFRA summary](https://wan-ifra.org/2016/01/lets-say-goodbye-to-pointless-ill-timed-endless-push-notifications/)
- Urban Airship / Airship (2017): across 63 million new app users, 95% of opted-in users who received no push in their first 90 days churned; moving from zero to weekly pushes was associated with 2 to 6 times higher 90-day retention depending on platform and report; media apps sending daily or more retained about twice as well as those sending weekly. Correlational: apps and users were not randomised to frequency. [Vendor, Obs] — [Airship benchmark PDF](https://grow.urbanairship.com/rs/313-QPJ-195/images/WP_App_Retention_Rates_Benchmarks.pdf); [Airship release](https://www.airship.com/newsroom/new-urban-airship-mobile-app-retention-study/)
- Braze: 25% of respondents in its survey said they opted out because they got too many pushes; a separate internal exercise was based on only 14 staff. [Vendor, very low quality] — [Braze](https://www.braze.com/resources/articles/opt-out-of-push-notifications-why-users-do-it)
- Chrome (October 2025): after testing automatic removal of notification permission from sites with high volume and very low engagement, Google reported "a significant reduction in notification overload with only a minimal change in total notification clicks". No sample size or method given. [Vendor] — [Chrome blog](https://blog.google/chromium/automatic-notification-permission/)

### Inferences
- Every platform that published on this ended up with relevance-ranked sending under a per-user budget, not a single fixed cap for everyone. A fixed "1 per day" rule is a ceiling of the same kind but without the per-user tuning.
- The retention benchmarks (Airship) and the fatigue surveys (Localytics) point in opposite directions because one measures what engaged users receive and the other what people say annoys them; neither is causal. Only Wohllebe 2021, LinkedIn's random-drop test and the Meta test randomised volume.
- The LinkedIn and Meta results together suggest the short-term cost of sending less is small (LinkedIn: −2.6% page views for halving email) and may reverse over a year (Meta), while the complaint reduction is large (−45% relative).

### Gaps
- No vendor or platform publishes an opt-out or uninstall curve by weekly push count from randomised data.
- Leanplum, OneSignal and recent (2024–2026) Airship or Braze frequency-versus-opt-out figures were not located; MoEngage and Accengage figures surfaced only second-hand without periods or methods, so I have not used them.
- Meta's post gives no effect sizes, and the original was readable only through a repost.

## 6. Digests and summaries

### Takeaway
There is no controlled evaluation of weekly digests or weekly reviews in productivity, habit or health apps, and none of the operating-system summary features. Support for the digest idea is indirect: the three-times-a-day batching trial, email-batching studies, and evidence that weekly health emails keep being opened for months.

### Cited Findings
- iOS Scheduled Summary (introduced in iOS 15, 2021) is off by default and I found no peer-reviewed or vendor evaluation of it. One long-term reviewer wrote after three months that it "only helped me ignore notifications more" and that they rarely opened the summary. [Anecdote] — [MacStories iOS 15 review](https://macstories.net/stories/ios-and-ipados-15-the-macstories-review/5)
- One self-experimenter who routed non-essential notifications into scheduled summaries reported daily screen time falling from about 5 hours to about 4. Single person, no control. [Anecdote] — [Toward Wellth](https://towardwellth.substack.com/p/right-swipe-study-results-screen)
- Worksite health emails (JMIR, 2006): participants received weekday emails for 26 weeks, and about three-quarters were still opening at least one email a week at week 26. [Obs] — [JMIR 2006;8(1):e3](https://www.jmir.org/2006/1/e3)
- Periodic email prompts in a Dutch health app (JMIR mHealth and uHealth, 2023): about 22,800 users over 78 weeks; receiving and opening a prompt was associated with a small rise in the probability of being active that week, larger when the email matched the programme's goals. [Obs] — [JMIR mHealth 2023;11:e43033](https://mhealth.jmir.org/2023/1/e43033)
- User-initiated deferral (Weber et al., MobileHCI 2018): a year-long in-the-wild study with 295 active users of a snooze app. Snoozing was used mostly for notifications about people and events; most were deferred to later the same day or the next morning, and deferrals of more than two days were the exception. [Obs] — [Weber et al. 2018](https://www.interruptions.net/literature/Weber-MobileHCI18.pdf)
- Delivery at natural breakpoints instead of immediately (Yahoo! JAPAN, UbiComp 2017): production test with 687,840 users over 21 days; delaying notifications a few minutes to a detected break in activity cut response time by 49.7% and raised click rates. [RCT at scale, Vendor-run] — [Tsubouchi & Okoshi 2017](https://interruptions.net/literature/Tsubouchi-UbiComp17.pdf)
- Shopper survey: participants valued being able to choose a frequency cap such as "no more than one per week". Qualitative. [Survey] — [Baymard Institute](https://baymard.com/blog/newsletter-frequency)
- A secondary blog (surfaced in search, not opened) claims Spotify moved new-release alerts to a weekly digest and saw better long-term engagement; I could not trace this to a primary source. [Unverified] — [Statsig blog](https://www.statsig.com/perspectives/push-experiments-engagement)

### Inferences
- Batching evidence (Fitz 2019; Kushlev & Dunn 2015) concerns batches delivered one to three times a day. Extrapolating to a once-a-week digest is not supported by any data either way.
- When people control deferral themselves they rarely push things more than two days out (Weber 2018), which suggests a weekly digest is a longer delay than users choose for things they care about, and that a snooze button should offer short horizons as well as "next week".
- The only durability data for a weekly-cadence message (about 75% still opening at 26 weeks) comes from a 2006 workplace programme with employer backing, so it is an optimistic bound.

### Gaps
- No open-rate, read-rate or action-rate data for weekly review or weekly digest messages in to-do, habit or companion apps.
- No evaluation of Android notification digests, iOS Scheduled Summary, or Slack/Teams digest features was found.
- No evidence on keep/snooze/drop buttons inside a digest.

## 7. What makes a notification worth the interruption

### Takeaway
Across large log studies and experience-sampling work, notifications about people and events and from communication apps are valued most, and usefulness of content and who sent it predict acceptance better than volume does. Quantitative detail from the specific papers named in the brief was only partly retrievable.

### Cited Findings
- Sahami Shirazi et al. (CHI 2014): close to 200 million notifications from more than 40,000 users with subjective ratings. Importance differed by type; users valued notifications from messaging apps and notifications "that include information about people and events". Abstract-level only. [Obs] — [Sahami Shirazi et al. 2014](https://doi.org/10.1145/2556288.2557189)
- Mehrotra et al. (CHI 2016, "My Phone and Me"): 20 users, 10,372 notifications, 474 in-situ questionnaire responses; examined alert modality, sender–recipient relationship, presentation, ongoing task, perceived disruption and response time. Later papers by the same group summarise the result as receptivity depending on content type and sender, with users mostly dismissing notifications that are not useful or relevant. Specific coefficients not retrieved. [Obs] — [Mehrotra et al. 2016](https://discovery.ucl.ac.uk/1502364/1/CHI16.pdf)
- Visuri et al. (International Journal of Human-Computer Studies 128:72–85, 2019): 40 users in the wild labelled notification content importance; four user types emerged based on daily volume and interaction choices, and prediction of unwanted notifications improved when individual content preferences were included. Abstract-level only. [Obs] — [Visuri et al. 2019](https://doi.org/10.1016/j.ijhcs.2019.03.001)
- Conversion by category (Pielot, Vradi & Park 2018): person-to-person messages opened about 64–66% of the time versus 15–27% for email, social and other apps. — [Pielot, Vradi & Park 2018](https://www.interruptions.net/literature/Pielot-MobileHCI18.pdf)
- Medication-reminder meta-analysis (Finitsis et al., PLoS ONE, 2014; 8 trials, 9 interventions): effects were larger when messages were personalised, allowed a reply (two-way), and were timed to the person's own dosing schedule. [Meta] — [Finitsis et al. 2014](https://pmc.ncbi.nlm.nih.gov/articles/PMC3914915)
- Meta's fewer-notifications test kept only notifications predicted to be rated 5 of 5 by users, i.e. selection was on predicted personal value. [Vendor] — [Analytics at Meta](https://exde601e.blogspot.com/2023/01/notifications-why-less-is-more.html)
- Tailored pushes in a well-being app were more effective at midday on weekends (11.8% more likely to engage, 90% CI 1.02–1.13) than on weekdays (2.5%, not significant). [MRT] — [Bidargaddi et al. 2018](https://doi.org/10.2196/10123)

### Inferences
- "About a person or an event" and "from someone I know" are the two properties that recur across independent datasets (Sahami Shirazi 2014; Pielot 2018; Weber 2018 on what gets snoozed rather than dismissed). An opportunity nudge tied to a real outside event fits the first; a companion with a consistent personal voice may approximate the second, but no study tests whether an AI sender is treated like a person.

### Gaps
- Per-category importance ratings from Sahami Shirazi 2014 and detailed results from Weber et al. 2016 (multi-device) and Mehrotra 2015 were not retrieved.
- No study isolates actionability (for example, buttons in the notification) as a factor in perceived worth.

## 8. User control and back-off after ignored notifications

### Takeaway
Evidence that frequency settings, quiet hours or snooze reduce disabling is thin and mostly indirect. I found no controlled study of back-off policies after consecutive dismissals; the nearest data point is Chrome's self-reported test of removing permission from high-volume, low-engagement senders.

### Cited Findings
- Fitz et al. attribute part of the anxiety in their no-notification arm to the app not letting people override it, and contrast it with Kushlev et al. (2016), where participants could switch back at will; they warn against "paternalistic interventions". Authors' interpretation, not a tested contrast. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016)
- After one day without notifications, 22 of 30 participants planned selective disabling or Do Not Disturb, and 13 were still doing so two years later. — [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314)
- With a snooze tool available for a year, 295 users deferred mainly people and event notifications, mostly to the same day or next morning. — [Weber et al. 2018](https://www.interruptions.net/literature/Weber-MobileHCI18.pdf)
- An unsubscribe is permanent loss of the channel for that message type, which is why LinkedIn weighed complaints heavily against short-term engagement. [Vendor] — [LinkedIn 2016](https://www.linkedin.com/blog/engineering/archive/less-is-more-optimizing-email-volume-part-1)
- Chrome's automatic revocation for sites with high notification volume and very low engagement cut overload with "only a minimal change in total notification clicks"; users are told and can re-grant. [Vendor] — [Chrome blog, October 2025](https://blog.google/chromium/automatic-notification-permission/)
- Duolingo is widely reported to stop practice reminders after a run of ignored ones with a final "these reminders don't seem to be working" message; the seven-day figure comes from a low-authority case-study page and no test result for the policy is published. [Unverified] — [case-study page](https://sites.google.com/view/case-vise/case-studies/duolingo)
- An unsourced vendor blog claims frequency controls cut unsubscribes by up to 37%; the underlying studies are not named. [Vendor, unverified] — [email-tool blog](https://cdmsuiteinsights.beehiiv.com/p/cut-email-fatigue-by-37-here-s-the-data-aee8113cdc10d437)

### Inferences
- The Chrome result implies that notifications sent to people who have stopped responding contribute almost nothing, so backing off after repeated non-response costs little. That is an inference from one self-reported platform test, not a tested back-off rule.

### Gaps
- No randomised evidence that offering frequency settings, quiet hours or snooze lowers the rate of disabling notifications or uninstalling.
- No evidence on what number of consecutive ignored nudges should trigger a pause, or how long the pause should be.

## 9. Ideal frequency for reminders about a person's own tasks or goals

### Takeaway
No dose-response curve exists for to-do or personal-goal reminders. The nearest evidence is medication and health-behaviour reminders, where weekly or less-than-daily messages matched or beat daily ones in HIV adherence trials, while two small app studies found daily notifications did not deter engagement.

### Cited Findings
- Pop-Eleches et al. (AIDS, 2011): Kenyan RCT over 48 weeks. 53% of patients receiving weekly SMS reminders reached at least 90% adherence versus 40% of controls (p = 0.03). Daily reminders gave 41% versus 40% (p = 0.92). The authors said further study was needed to explain why weekly worked and daily did not. [RCT] — [aidsmap report](https://www.aidsmap.com/news/jan-2011/weekly-text-messages-improve-hiv-drug-adherence-kenya)
- Finitsis, Pellowski & Johnson (PLoS ONE, 2014): meta-analysis of 8 trials (9 interventions) of text messaging for antiretroviral adherence; overall OR = 1.39 (95% CI 1.18–1.64). Effects were larger when messages were sent less often than daily. [Meta] — [Finitsis et al. 2014](https://pmc.ncbi.nlm.nih.gov/articles/PMC3914915)
- A Uganda pilot's review of the field: weekly and twice-weekly reminders improved adherence in Kenya and Nigeria, while daily texts showed no benefit in Kenya, India, Cameroon or Brazil. [Review within a pilot study; taken from a search summary, paper not opened, so the link may not be the exact source] — [BMC Med Inform Decis Mak 2017](https://link.springer.com/10.1186/s12911-017-0413-9)
- Morrison et al. (PLoS ONE, 2017): exploratory trial, 77 users of a stress-management app randomised to "intelligent", daily, or occasional notifications. Daily and intelligent groups viewed and acted on more notifications than the occasional group; the authors concluded frequent notifications "may encourage greater exposure to intervention content without deterring engagement" and that the hypotheses need confirmatory testing. [RCT, exploratory, small] — [Morrison et al. 2017](https://pmc.ncbi.nlm.nih.gov/articles/PMC5207732)
- Pirolli et al. (JMIR, 2017): 64 adults over 28 days with a health-behaviour app, randomised to 7 or 14 reminders in total and to different spacing; frequency interacted with spacing on daily goal success. Abstract statistics looked internally inconsistent in the summary I saw, so I report the design only. [RCT, small] — [Pirolli et al. 2017](https://jmir.org/2017/11/e397)
- Headache diary app (JMIR mHealth and uHealth, 2026): a 14-day outage of daily reminders served as a natural experiment, and diary adherence was better with reminders. Retrospective cohort. [Obs] — [JMIR mHealth and uHealth 2026, e73259](https://mhealth.jmir.org/2026/1/e73259/PDF)
- At most one notification a day kept its effect for about a month in Drink Less (3.5-fold, no significant change over time), whereas with up to five a day in HeartSteps the effect diminished over six weeks (66% at the start versus 14% averaged over the study). [MRT] — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342); [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)

### Inferences
- The health-reminder literature spans roughly one per week to five per day. Within it, about one per week to one per day is where benefits were found without a measured decline, and multiple per day is where decline was observed. This is a pattern across unlike studies, not a tested curve.
- Medication reminders concern a repeated, externally prescribed behaviour. A person's own one-off wishes ("I want to try that restaurant") differ, so transfer is uncertain.

### Gaps
- No study of reminder frequency in to-do, task-management or personal-goal apps was found.
- No systematic review focused on reminder frequency as the manipulated variable was found.
- No evidence for or against "about 3 per week" as a frequency for any reminder type.

## 10. What this does and does not support for the proposed nudge strategy

### Takeaway
The structure of the proposal (time-critical items immediate, everything else batched, user control, low caps) is consistent with the evidence, but none of the specific numbers (1 per day, about 3 per week, one weekly digest) is backed by a study. The proposal's worst-case total is higher than the current rule, which the evidence does not favour.

### Cited Findings
- Time-critical and event-related notifications are the ones users value and defer rather than discard. — [Sahami Shirazi et al. 2014](https://doi.org/10.1145/2556288.2557189); [Weber et al. 2018](https://www.interruptions.net/literature/Weber-MobileHCI18.pdf)
- Withholding everything raises worry about missing things; batching at a few predictable times improved well-being in one RCT. — [Fitz et al. 2019](https://doi.org/10.1016/j.chb.2019.07.016); [Pielot & Rello 2017](https://arxiv.org/abs/1612.02314); [Dekker et al. 2024](https://doi.org/10.1080/15213269.2024.2334025)
- Self-reported tolerance for pushes from a single app: about 10% say one a week would make them disable notifications; 37–46% say two to five a week would; churn intentions peak at six to ten a week. [Vendor, Survey] — [MarketingProfs summary of Localytics](https://www.marketingprofs.com/charts/2018/33486/how-many-mobile-push-notifications-are-too-many); [WAN-IFRA](https://wan-ifra.org/2016/01/lets-say-goodbye-to-pointless-ill-timed-endless-push-notifications/)
- At most one notification a day held its effect for about a month in one MRT; with up to five a day the effect diminished over six weeks in another (66% at the start versus 14% averaged). — [Bell et al. 2023](https://mhealth.jmir.org/2023/1/e38342); [Klasnja et al. 2019](https://pubmed.ncbi.nlm.nih.gov/30192907/)
- Weekly reminders outperformed daily in HIV adherence trials. — [Finitsis et al. 2014](https://pmc.ncbi.nlm.nih.gov/articles/PMC3914915)
- Repeats of the same alert and several alerts in one sitting each reduce acceptance; varied presentation sustains response. — [Ancker et al. 2017](https://pmc.ncbi.nlm.nih.gov/articles/PMC5387195); [Vance et al. 2018](https://misq.org/tuning-out-security-warnings-a-longitudinal-examination-of-habituation-through-fmri-eye-tracking-and-field-experiments.html)
- Platforms that cut volume by relevance held or improved engagement and cut complaints. [Vendor] — [Analytics at Meta](https://exde601e.blogspot.com/2023/01/notifications-why-less-is-more.html); [LinkedIn 2016](https://www.linkedin.com/blog/engineering/archive/less-is-more-optimizing-email-volume-part-1)

### Inferences
- (a) Deadline reminders always sent: consistent with the evidence that time-critical, event-bound notifications are valued, and with the finding that hiding things people would want raises anxiety. Not supported: repeating a reminder for the same deadline; the clinical data suggest repeats about the same item are where acceptance drops. No study tests an "always send" exemption.
- (b) Opportunity nudges at most 1 per day: the type (tied to a real outside event) matches what users rate as important. The number is unsupported. One per day sustained its effect for a month in one alcohol-app trial, but seven a week from one app sits in the band where survey respondents most often say they would quit, and weekly beat daily in the HIV reminder trials. The evidence fits treating 1 per day as a ceiling that is rarely reached, with a relevance bar deciding whether anything is sent.
- (c) Quick-action reminders at about 3 per week: there is no evidence for or against this number. It falls in the two-to-five-a-week band where 22–46% of survey respondents (depending on wave and question) said they would disable or stop using a generic app, so it is not a self-evidently safe level for low-value content.
- (d) One weekly digest: indirectly supported as a way to cut interruptions while keeping the person informed, and a weekly cadence was still being opened by about three-quarters of recipients at six months in one old workplace study. Not supported by any direct test: no data on whether weekly digests in productivity apps get read, and the one long-term account of an OS summary feature says it made notifications easier to ignore. The keep/snooze/drop buttons have no evidence base either way.
- Combined volume: the proposal allows up to 7 opportunity nudges, about 3 quick-action reminders and 1 digest a week, plus deadlines — 11 or more a week in the worst case, against 7 under the current one-per-day rule. Nothing in the evidence argues for raising total volume; the platform experiments and the dose pattern in the MRTs point the other way. A shared weekly budget across (b) and (c) would be closer to Pinterest's published per-user weekly budget approach.
- Design details that do have support: deliver at predictable times (Fitz), vary wording and format across nudges (Vance; Duolingo), avoid stacking several nudges together (Ancker), let the person override or adjust (Fitz discussion; Pielot & Rello follow-up), and offer short snooze horizons such as later today or tomorrow morning (Weber).

### Gaps
- No evidence base exists for a personal AI companion's proactive messages specifically; everything above is transferred from marketing pushes, phone-wide notification studies, health reminders, clinical alerts and security warnings.
- The specific caps (1 per day, 3 per week, 1 digest per week) can only be validated by the app's own measurement of response, dismissal and mute rates.
- No evidence on how deadline reminders interact with fatigue from the other nudge types.
