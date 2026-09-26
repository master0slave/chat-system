"use client";

import Link from "next/link";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { clearSession, loadSession, type Session } from "@/lib/session";

export function Header() {
  const pathname = usePathname();
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);

  // Re-read on every navigation, because logging in happens on another page.
  useEffect(() => setSession(loadSession()), [pathname]);

  function logout() {
    clearSession();
    setSession(null);
    router.push("/login");
  }

  return (
    <header className="header">
      <Link href="/cases">Support Chat</Link>
      {session && (
        <div className="row">
          <span data-testid="current-user">
            {session.user.name} <span className="muted">({session.user.role})</span>
          </span>
          <button className="secondary" data-testid="logout" onClick={logout}>
            Log out
          </button>
        </div>
      )}
    </header>
  );
}
