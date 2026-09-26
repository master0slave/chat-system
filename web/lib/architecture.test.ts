import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { describe, expect, it } from "vitest";

// ADR 0001 rule 6: only web/lib talks to the backend. Pages and components go through it.
const forbidden = [
  { pattern: /\bfetch\s*\(/, why: "use api from @/lib/api" },
  { pattern: /new\s+WebSocket\s*\(/, why: "use connectEvents from @/lib/socket" },
  { pattern: /\b(sessionStorage|localStorage)\b/, why: "use @/lib/session" },
];

const root = join(__dirname, "..");

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) return sourceFiles(path);
    return /\.(ts|tsx)$/.test(name) ? [path] : [];
  });
}

describe("dependency rule", () => {
  it("keeps backend calls inside web/lib", () => {
    const violations = sourceFiles(join(root, "app")).flatMap((file) => {
      const text = readFileSync(file, "utf8");
      return forbidden.filter((f) => f.pattern.test(text)).map((f) => `${relative(root, file)}: ${f.why}`);
    });

    expect(violations).toEqual([]);
  });
});
