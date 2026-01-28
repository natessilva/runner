# Runner - Feature Roadmap

Future enhancements to take the app to the next level.

## High Impact, Moderate Effort

### [x] Activity Detail View

Press enter on an activity to see detailed analysis:

- Pace & HR graph over time (visualize where you fatigued)
- Mile/km splits with HR for each split
- HR zone distribution breakdown
- ~~Decoupling visualization (first half vs second half)~~
- ~~Grade-adjusted pace analysis for hilly runs~~

### [ ] HR Zone Analysis

Weekly and monthly breakdown of time spent in each HR zone:

- Zone 1 (Recovery): <60% max HR
- Zone 2 (Aerobic): 60-70% max HR
- Zone 3 (Tempo): 70-80% max HR
- Zone 4 (Threshold): 80-90% max HR
- Zone 5 (VO2max): >90% max HR

Key insight: Most runners don't do enough easy running. This helps ensure 80/20 polarization.

### [x] Trend Comparisons

Add comparison views:

- This week vs last week
- This month vs last month
- This month vs same month last year
- Rolling 30-day comparisons

Show deltas for: mileage, avg HR, avg cadence, avg EF, run count.

## High Impact, More Effort

### [ ] Race Time Predictions

Use EF trends and recent workout data to estimate race times:

- 5K prediction
- 10K prediction
- Half marathon prediction
- Marathon prediction

Could use Jack Daniels' VDOT tables or similar methodology.

### [ ] Training Load Calendar

Heat map view of the year showing daily training load:

- Similar to GitHub contribution graph
- Color intensity = TRIMP value
- Click on a day to see activities
- Visual patterns for training blocks and rest weeks

### [ ] Aerobic Base Score

Track what percentage of running is truly aerobic:

- Calculate time below MAF HR (180 - age) or custom threshold
- Weekly/monthly aerobic percentage
- Target: 80%+ of volume should be easy
- Alert when doing too much intensity

## Quick Wins

### [ ] Personal Records

Track and display PRs for common distances:

- 1 mile, 5K, 10K, half marathon, marathon
- Auto-detect from activity data
- Show date achieved and conditions
- Highlight when PR is broken

### [ ] Weekly Summary Export

Generate a text/markdown report for training logs:

- Total mileage, time, runs
- Average pace, HR, cadence
- Notable workouts
- Fitness metrics summary
- Copy to clipboard or save to file

## Nice to Have

### [ ] Training Phases

Mark periods with training phase labels:

- Base building
- Build phase
- Peak/taper
- Recovery

Helps contextualize metrics within training cycles.

### [ ] Route/Course Comparison

Compare performance on the same route over time:

- Match activities by GPS similarity
- Show EF/pace/HR trends for that specific route
- Great for seeing fitness gains on regular routes

### [ ] Fitness Projection

Project future fitness based on planned training:

- "If you maintain X miles/week, CTL will be Y in 4 weeks"
- Help plan tapers for races
- Show when you'll reach target fitness

### [ ] Sleep/Recovery Integration

If Strava has sleep data or integrate with other sources:

- Correlate recovery with performance
- Suggest rest days based on fatigue

---

## Completed Features

- [x] Dashboard with EF, CTL/ATL/TSB metrics
- [x] Weekly mileage, HR, cadence, EF charts
- [x] Activities list with pagination
- [x] Period stats (weekly/monthly aggregates)
- [x] Strava sync with rate limiting
- [x] Scrollable dashboard
- [x] Help screen with metrics explanations
- [x] Monday-based weeks
