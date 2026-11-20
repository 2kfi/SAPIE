# SAPIE — Development Notes

## Deferred items after MF1

Carried forward from the MR1 review and MF1 fixes at dev head `0bba875`.
None of these block the release; they are known limits, not failures found in testing.

### 1. Mutable mirror refs

The C++ client's `friends()` and `lobbies()` return references to internal lists rather than
copies. In theory another thread could change a list while someone reads it. In practice every
caller is serialized behind a global lock, so there is no observed race. Fixing it properly means
rewriting several Steam interface methods to use copies — broad churn for a hypothetical problem.
Left as a labelled follow-up.

### 2. `StatsGet.names` ignored

The wire message has a field where a caller can say "only send me these named stats." The server
ignores that list and returns everything for that user/app. The result is a superset, so nothing
breaks today — but the field is dead plumbing, and asking for one stat still returns all of them.

### 3. Hidden achievements in unlock pushes

We fixed the *query* leak: a friend can no longer ask the server for your hidden achievements. But
when you actually unlock one, the server still pushes that unlock to friends. So a friend can learn
a hidden achievement's name and unlock time through the push, even though they cannot query it.
It is a smaller version of the privacy issue MF1 already closed.

### 4. No live end-to-end run

This is the real gap. Everything so far was proven in pieces: in-process Go harnesses, stubbed C++
smoke messages, isolated builds. We have not launched two real C++ clients against the real Go
server, run an actual old Steam game with the replacement DLL, or clicked through the web UI in a
browser. Any of those could expose an integration bug that none of our tests cover.

Items 1–3 are small, understood limitations. Item 4 is the honest confidence gap: "nothing blocks
the merge" is not the same as "proven on a real machine." Mains were merged from dev for the 0.1
release; see the release tag for the exact commit.

### Verification caveat

None of the four items were re-verified live; they come from worker reports and the merged-head test
suite. Main risk: the real two-client path stays unproven until item 4 is run.
