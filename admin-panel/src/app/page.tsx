"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-context";

export default function Home() {
  const { loading, accessToken } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (loading) return;
    router.replace(accessToken ? "/students" : "/login");
  }, [loading, accessToken, router]);

  return null;
}
