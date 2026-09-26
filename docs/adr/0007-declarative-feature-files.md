# ADR 0007: Declarative Feature Files

## Status

Accepted

## Context

Feature files that list clicks and field names ("imperative" style) are long, break when the UI changes, and hide the business rule. For example:

```gherkin
When I click "Login"
And I type "Ann" into "name"
And I click "New case"
```

This says nothing about *why* Ann is there.

## Decision

1. Feature files describe behaviour in business language: who, what, and the outcome.

   ✅ `Given customer "Ann" has asked "How do I reset my password?"`
   ❌ `When I type "How do I reset my password?" into the "question" field`

2. Each feature file starts with `As a / I want / So that`.

3. Each user story in the spec has one feature file in `e2e/cucumber/features/`.

4. UI details live in Page Objects (ADR 0006), never in `.feature` files.

## Consequences

### Positive

- Non-developers can read and review the scenarios.
- Scenarios survive UI redesigns.

### Negative

- Step definitions do more work, because one step may cover several UI actions.
