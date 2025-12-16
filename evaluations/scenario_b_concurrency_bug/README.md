# Scenario B: The Concurrency Bug

## Background
You are debugging a worker pool implementation that processes jobs. The code is supposed to count how many results are greater than 10.

## Task
The current implementation has a bug related to concurrent access to the `count` variable. 
1.  Identify the data race.
2.  Fix the bug using a thread-safe mechanism (e.g., `sync.Mutex` or `atomic`).

## Instructions for Agent
*   Analyze `main.go`.
*   Fix the race condition.
*   Ensure the program still runs and outputs the count.
