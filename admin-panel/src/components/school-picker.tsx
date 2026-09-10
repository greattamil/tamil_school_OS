"use client";

import { useState } from "react";
import { useAuth } from "@/lib/auth-context";
import type { SchoolRole } from "@/lib/api";

// Shown when a staff member holds a role at more than one school (PRD 3.2.1: a
// trust running a matriculation school alongside a feeder campus). Selecting a
// school exchanges the current token for one scoped to it -- never a parameter
// change on the existing token.
export function SchoolPicker({ schools }: { schools: SchoolRole[] }) {
  const { pickSchool } = useAuth();
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function choose(school: SchoolRole) {
    setPending(true);
    setError("");
    try {
      await pickSchool(school);
    } catch {
      setError("Could not switch to that school. Please try again.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mx-auto mt-16 max-w-sm px-4">
      <h1 className="text-lg font-semibold text-zinc-900">Choose a school</h1>
      <p className="mt-1 text-sm text-zinc-500">You hold a role at more than one school.</p>
      {error && <p className="mt-3 text-sm text-red-600">{error}</p>}
      <ul className="mt-4 divide-y divide-zinc-200 rounded-md border border-zinc-200">
        {schools.map((s) => (
          <li key={s.school_id}>
            <button
              type="button"
              disabled={pending}
              onClick={() => choose(s)}
              className="flex w-full items-center justify-between px-4 py-3 text-left text-sm hover:bg-zinc-50 disabled:opacity-50"
            >
              <span className="font-medium text-zinc-900">{s.school_name}</span>
              <span className="text-zinc-500 capitalize">{s.role.replace("_", " ")}</span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}
