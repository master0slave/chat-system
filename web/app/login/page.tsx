"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { api } from "@/lib/api";
import { saveSession } from "@/lib/session";
import type { Role } from "@/lib/types";

export default function LoginPage() {
  const router = useRouter();
  const [name, setName] = useState("");
  const [role, setRole] = useState<Role>("customer");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      saveSession(await api.login(name, role));
      router.push("/cases");
    } catch (err) {
      setError(err instanceof Error ? err.message : "Login failed");
      setBusy(false);
    }
  }

  return (
    <div className="card">
      <h1>Log in</h1>
      <p className="muted">No password in this version: pick a name and a role (ADR 0005).</p>
      <form onSubmit={submit}>
        <p>
          <label>
            Name
            <input type="text" data-testid="login-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </label>
        </p>
        <p className="row">
          <label>
            <input
              type="radio"
              name="role"
              data-testid="login-role-customer"
              checked={role === "customer"}
              onChange={() => setRole("customer")}
            />{" "}
            Customer
          </label>
          <label>
            <input type="radio" name="role" data-testid="login-role-agent" checked={role === "agent"} onChange={() => setRole("agent")} />{" "}
            Support agent
          </label>
        </p>
        {error && (
          <p className="error" data-testid="login-error">
            {error}
          </p>
        )}
        <button type="submit" data-testid="login-submit" disabled={busy || name.trim() === ""}>
          Log in
        </button>
      </form>
    </div>
  );
}
