"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useState } from "react";
import { selectSchool as apiSelectSchool, staffLogin, type SchoolRole } from "./api";

const STORAGE_KEY = "tamil-school-os.admin.session";

interface StoredSession {
  accessToken: string;
  refreshToken: string;
  schools: SchoolRole[];
  activeSchool: SchoolRole | null;
}

interface AuthState extends StoredSession {
  loading: boolean;
  login: (identifier: string, password: string) => Promise<void>;
  pickSchool: (school: SchoolRole) => Promise<void>;
  logout: () => void;
}

const AuthContext = createContext<AuthState | null>(null);

// A stable per-browser device id, sent as device_id on login (PRD 6.2: sessions are
// listed and individually revocable per device). Not a security boundary -- just a
// label so "all sessions" in a future admin screen means something to a human.
function getOrCreateDeviceId(): string {
  if (typeof window === "undefined") return "server";
  const key = "tamil-school-os.device-id";
  let id = window.localStorage.getItem(key);
  if (!id) {
    id = `admin-panel-${crypto.randomUUID()}`;
    window.localStorage.setItem(key, id);
  }
  return id;
}

function loadSession(): StoredSession | null {
  if (typeof window === "undefined") return null;
  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as StoredSession;
  } catch {
    return null;
  }
}

function saveSession(session: StoredSession | null) {
  if (typeof window === "undefined") return;
  if (session) {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
  } else {
    window.localStorage.removeItem(STORAGE_KEY);
  }
}

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [session, setSession] = useState<StoredSession>({
    accessToken: "",
    refreshToken: "",
    schools: [],
    activeSchool: null,
  });
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    // Reading localStorage can only happen after hydration (the server render has
    // no window), so this genuinely needs an effect rather than a lazy useState
    // initializer -- an initializer would read real client state on the very
    // first client render and mismatch the server's necessarily-empty render,
    // which is exactly the hydration bug this two-pass (loading -> real state)
    // pattern exists to avoid.
    const stored = loadSession();
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (stored) setSession(stored);
    setLoading(false);
  }, []);

  const login = useCallback(async (identifier: string, password: string) => {
    const deviceId = getOrCreateDeviceId();
    const result = await staffLogin(identifier, password, deviceId);
    const activeSchool = result.schools.length === 1 ? result.schools[0] : null;
    const next: StoredSession = {
      accessToken: result.access_token,
      refreshToken: result.refresh_token,
      schools: result.schools,
      activeSchool,
    };
    setSession(next);
    saveSession(next);
  }, []);

  const pickSchool = useCallback(
    async (school: SchoolRole) => {
      const deviceId = getOrCreateDeviceId();
      const result = await apiSelectSchool(session.refreshToken, school.school_id, deviceId);
      const next: StoredSession = {
        accessToken: result.access_token,
        refreshToken: result.refresh_token,
        schools: result.schools,
        activeSchool: school,
      };
      setSession(next);
      saveSession(next);
    },
    [session.refreshToken],
  );

  const logout = useCallback(() => {
    const empty: StoredSession = { accessToken: "", refreshToken: "", schools: [], activeSchool: null };
    setSession(empty);
    saveSession(null);
  }, []);

  const value = useMemo<AuthState>(
    () => ({ ...session, loading, login, pickSchool, logout }),
    [session, loading, login, pickSchool, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
