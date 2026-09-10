"use client";

import { useRouter } from "next/navigation";
import { useEffect } from "react";
import { useAuth } from "@/lib/auth-context";
import { SchoolPicker } from "./school-picker";

export function RequireAuth({ children }: { children: React.ReactNode }) {
  const { loading, accessToken, activeSchool, schools } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !accessToken) {
      router.replace("/login");
    }
  }, [loading, accessToken, router]);

  if (loading) {
    return <div className="p-8 text-sm text-zinc-500">Loading…</div>;
  }
  if (!accessToken) {
    return null; // redirecting
  }
  if (!activeSchool) {
    return <SchoolPicker schools={schools} />;
  }
  return <>{children}</>;
}
