You are a **senior backend engineer and distributed systems developer**.

Your task is to implement a **production-quality multiplayer rating system library in Go (Golang)**.

Your output MUST be a **complete, runnable Go module**, including all source files, tests, documentation, and an example program.

You must follow the requirements strictly.

If something is unclear, choose a **reasonable engineering decision and document it**.

The final project must **compile and run without modification**.

---

# Core Goal

Implement a reusable **multiplayer rating system** inspired by **Elo and Glicko-like systems** used in competitive games.

The library must support:

* 1v1 matches
* team matches (2v2, 3v3, 5v5)
* matches with multiple teams
* rating updates for each player
* dynamic rating volatility
* inactivity handling
* match quality estimation

The code should be structured as a **general-purpose Go package** usable by different games.

---

# Hard Requirements

The implementation MUST include the following features.

---

# 1. Player Model

Each player must include:

* ID
* Rating
* Rating Deviation (RD / uncertainty)
* GamesPlayed
* LastActive timestamp

New players must start with configurable defaults.

Example default parameters:

* default_rating
* default_rd

---

# 2. Match Model

Matches must support:

* any number of teams
* any number of players per team
* ranking results

Example:

Team A (Rank 1)
Team B (Rank 2)
Team C (Rank 3)

Ranks determine match outcome.

---

# 3. Team Rating

You must implement a **team rating function**.

Allowed approaches include:

* average player rating
* weighted rating
* another reasonable approach

You must document the reasoning in comments.

---

# 4. Expected Score Formula

Use the Elo expected score formula:

E(A) = 1 / (1 + 10^((RB - RA) / 400))

For multi-team matches:

* compare each team against all other teams
* aggregate expected outcomes

---

# 5. Dynamic K Factor

The rating update factor must depend on player state.

Requirements:

* higher volatility for new players
* lower volatility for very high rating players
* configurable values

Typical states:

provisional players
normal players
high rating players

---

# 6. Provisional Rating Phase

Players must be provisional for their first N matches.

Requirements:

* configurable provisional match count
* provisional players must have higher rating volatility
* rating must converge faster during early matches

---

# 7. Rating Deviation (Uncertainty)

Implement a **rating deviation concept inspired by Glicko**.

Requirements:

* new players start with high RD
* RD decreases with games played
* RD increases with inactivity
* RD must influence rating volatility

You must explain the chosen formula.

---

# 8. Inactivity Handling

Players inactive for a configurable time must experience:

* rating decay OR
* RD increase

Minimum requirements:

* configurable inactivity period
* configurable decay amount
* prevent rating dropping below minimum

---

# 9. Match Quality

Implement a function:

MatchQuality(teamA, teamB)

Return a value between:

0.0 → extremely unfair match
1.0 → perfectly balanced

This will be used by matchmaking.

---

# 10. Rating Update Logic

The rating update algorithm must:

1. compute expected outcome
2. compute actual outcome
3. compute rating delta
4. update player ratings
5. update RD
6. update games played
7. update last active

The function must return:

map[playerID]ratingDelta

---

# 11. Configuration System

All tuning parameters must be configurable through a struct.

Example parameters:

* BaseK
* ProvisionalK
* ProvisionalMatches
* DefaultRating
* DefaultRD
* MinRD
* MaxRD
* InactivityPeriod
* DecayRate

No magic constants should exist inside logic.

---

# 12. Go Module Structure

You MUST output the project in a clean Go module layout.

Example:

project-root/

go.mod

rating/
config.go
player.go
team.go
match.go
rating.go
decay.go
quality.go
simulator.go

cmd/example/
main.go

tests/
rating_test.go

README.md

---

# 13. Unit Tests

Write comprehensive tests using Go's testing framework.

Tests must verify:

* winner gains rating
* loser loses rating
* team matches update all players
* provisional players move faster
* RD decreases after matches
* inactivity increases RD or decays rating
* match quality behaves correctly
* rating system remains stable under simulation

Tests must run with:

go test ./...

---

# 14. Simulation Tool

Implement a simulator that:

* generates random matches
* runs thousands of matches
* shows rating convergence

This is used to validate rating stability.

---

# 15. Example Program

Provide a runnable example that demonstrates:

* creating players
* creating teams
* running a match
* printing rating changes
* printing final ratings

Must run with:

go run ./cmd/example

---

# 16. Documentation

Provide a **complete README.md in English** explaining:

* system design
* formulas used
* configuration options
* example usage
* how to run tests
* how to run the simulator

---

# 17. Code Quality Requirements

The code must:

* compile successfully
* be idiomatic Go
* avoid unnecessary dependencies
* include clear comments explaining formulas
* separate concerns across files
* avoid duplicated logic

---

# Final Output Requirements

Your response must include:

1. The full project directory structure
2. Every source file with full contents
3. Unit tests
4. Example program
5. README.md

The code must be **copyable, buildable, and runnable immediately**.

Do not omit files.
Do not summarize code.
Do not provide pseudocode.
