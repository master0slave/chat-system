"use client";

import { useRouter } from "next/navigation";
import { useState, type FormEvent } from "react";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Spinner } from "@/components/ui/spinner";
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs";
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
    <Card className="login-card">
      <CardHeader>
        <CardTitle>Log in</CardTitle>
        <CardDescription>No password in this version: pick a name and a role (ADR 0005).</CardDescription>
      </CardHeader>
      <CardContent>
        <form className="login-form" onSubmit={submit}>
          <label className="form-field">
            <span>Name</span>
            <Input type="text" data-testid="login-name" value={name} onChange={(e) => setName(e.target.value)} autoFocus />
          </label>
          <Tabs value={role} onValueChange={(value) => setRole(value as Role)}>
            <TabsList className="role-tabs" aria-label="Choose your role">
              <TabsTrigger value="customer" data-testid="login-role-customer">Customer</TabsTrigger>
              <TabsTrigger value="agent" data-testid="login-role-agent">Support agent</TabsTrigger>
            </TabsList>
          </Tabs>
          {error && <Alert variant="destructive" data-testid="login-error"><AlertDescription>{error}</AlertDescription></Alert>}
          <Button type="submit" data-testid="login-submit" disabled={busy || name.trim() === ""}>
            {busy && <Spinner data-icon="inline-start" />}
            Log in
          </Button>
        </form>
      </CardContent>
    </Card>
  );
}
