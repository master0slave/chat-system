# ADR 0006: Page Object Pattern for E2E Tests

## Status

Accepted

## Context

E2E step definitions that use selectors directly break in many places when one screen changes. For example, if three steps use `page.locator('#send')`, renaming that button means editing all three.

## Decision

1. Each screen has one Page Object class in `e2e/cucumber/pages/` (`LoginPage`, `CaseListPage`, `CaseRoomPage`). It is the only code that knows that screen's selectors.

2. Page Object methods describe what a user does or sees, not how.

   ✅ `await caseRoom.sendMessage("Thanks")`
   ❌ `await page.fill('[data-testid="message-input"]', "Thanks"); await page.click('#send')` inside a step

3. Selectors use `data-testid` attributes or accessible roles, never CSS classes.

4. Step definitions in `e2e/cucumber/steps/` only call Page Objects and assertions.

5. Each actor (customer, agent) gets its own Playwright browser context, so one scenario can show real-time delivery between two people.

## Consequences

### Positive

- A UI change is fixed in one Page Object.
- Steps read like the feature files.

### Negative

- One more layer to write before the first E2E test runs.
