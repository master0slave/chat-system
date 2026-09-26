"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { loadSession, type Session } from "@/lib/session";

// useSession returns the logged-in session, or null while loading.
// Without a session it sends the browser to /login.
export function useSession(): Session | null {
  const router = useRouter();
  const [session, setSession] = useState<Session | null>(null);

  useEffect(() => {
    const s = loadSession();
    if (s) setSession(s);
    else router.replace("/login");
  }, [router]);

  return session;
}
