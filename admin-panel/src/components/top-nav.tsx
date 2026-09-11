"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useAuth } from "@/lib/auth-context";

const links = [
  { href: "/students", label: "Students" },
  { href: "/import", label: "Bulk import" },
  { href: "/fees", label: "Fees" },
];

export function TopNav() {
  const { accessToken, activeSchool, logout } = useAuth();
  const pathname = usePathname();

  if (!accessToken || !activeSchool) return null;

  return (
    <header className="border-b border-zinc-200 bg-white">
      <div className="mx-auto flex max-w-5xl items-center justify-between px-4 py-3">
        <div className="flex items-center gap-6">
          <span className="text-sm font-semibold text-zinc-900">{activeSchool.school_name}</span>
          <nav className="flex gap-4">
            {links.map((l) => (
              <Link
                key={l.href}
                href={l.href}
                className={`text-sm ${
                  pathname?.startsWith(l.href) ? "font-medium text-zinc-900" : "text-zinc-500 hover:text-zinc-700"
                }`}
              >
                {l.label}
              </Link>
            ))}
          </nav>
        </div>
        <button type="button" onClick={logout} className="text-sm text-zinc-500 hover:text-zinc-700">
          Log out
        </button>
      </div>
    </header>
  );
}
