# Senior Go Review Checklist

Criteria for "Senior+" quality code.

## Interface Design
*   **Consumer Defined:** Are interfaces defined where they are *used* (consumer), not where they are implemented? (Unless it's a foundational library like `io.Reader`).
*   **Small Interfaces:** Do interfaces have 1-3 methods? (Interface Segregation Principle).

## Concurrency
*   **No Uncontrolled Goroutines:** Does every `go func()` have a clear lifecycle? (Context cancellation, WaitGroup, or ErrGroup).
*   **No Mutex Copying:** Are structs with `sync.Mutex` passed by pointer?
*   **Channel Hygiene:** Are channels closed by the sender? Is `select` used to prevent deadlocks?

## API Design
*   **Return Concrete Types:** Functions should generally return struct pointers (`*Service`), not interfaces (`IService`), to allow consumers to define their own interfaces.
*   **Configuration:** uses `Functional Options` or a `Config` struct for complex constructors.

## Error Handling
*   **Typed Errors:** Uses `errors.Is` / `errors.As` support?
*   **Context:** Are errors wrapped with `fmt.Errorf("doing x: %w", err)`?
