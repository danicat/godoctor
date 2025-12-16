# Scenario A: The API Evolution

## Background
You are a software engineer working on a Go microservice that manages a catalog of products. The code is organized in a standard project layout.

## Task
The product team wants to add a `Category` to the product definition.
You need to:
1.  Add a `Category` field (string) to the `Product` struct in `internal/product/product.go`. Ensure it has a JSON tag.
2.  Update the `Save` method in `internal/product/product.go` to validate that `Category` is not empty (return error "missing category" if empty).
3.  Ensure the `cmd/server/main.go` handler doesn't need changes (it uses `json.Decoder`, so it should handle the new field automatically if the struct is updated), but check it to be sure.

## Instructions for Agent
*   Explore the codebase to understand the struct definition.
*   Apply the changes safely.
*   Verify your changes (you can run `go build ./...` inside `evaluations/scenario_a_api_evolution/workspace` if you wish, or trust the `edit_code` soft validation).
