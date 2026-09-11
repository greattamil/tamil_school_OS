"use client";

import Link from "next/link";
import { useCallback, useEffect, useState } from "react";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import { ApiError, duesAgeing, listStudents, formatPaise, type DuesRow, type Student } from "@/lib/api";

export default function FeesPage() {
  return (
    <RequireAuth>
      <FeesContent />
    </RequireAuth>
  );
}

function FeesContent() {
  const { accessToken, activeSchool } = useAuth();
  const [dues, setDues] = useState<DuesRow[]>([]);
  const [students, setStudents] = useState<Student[]>([]);
  const [search, setSearch] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const canSeeOfficeView = activeSchool?.role === "correspondent" || activeSchool?.role === "office_admin";

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [studentsResult, duesResult] = await Promise.all([
        listStudents(accessToken),
        canSeeOfficeView ? duesAgeing(accessToken) : Promise.resolve({ items: [] }),
      ]);
      setStudents(studentsResult.items);
      setDues(duesResult.items);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load fees data.");
    } finally {
      setLoading(false);
    }
  }, [accessToken, canSeeOfficeView]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  const filteredStudents = students.filter((s) => {
    if (!search.trim()) return false;
    const q = search.trim().toLowerCase();
    return s.name_english.toLowerCase().includes(q) || s.admission_number.toLowerCase().includes(q);
  });

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-zinc-900">Fees</h1>
        <nav className="flex gap-4 text-sm">
          {canSeeOfficeView && (
            <>
              <Link href="/fees/config" className="text-zinc-500 hover:text-zinc-700">
                Configure heads &amp; structures
              </Link>
              <Link href="/fees/concessions" className="text-zinc-500 hover:text-zinc-700">
                Concessions needing review
              </Link>
              <Link href="/fees/cash-drawer" className="text-zinc-500 hover:text-zinc-700">
                Cash drawer
              </Link>
              <Link href="/fees/annexure" className="text-zinc-500 hover:text-zinc-700">
                Regulatory export
              </Link>
            </>
          )}
        </nav>
      </div>

      <div className="mt-6 rounded-md border border-zinc-200 bg-white p-4">
        <label className="block text-sm font-medium text-zinc-700">Find a student to collect a payment or view dues</label>
        <input
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Search by name or admission number…"
          className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-zinc-500 focus:outline-none"
        />
        {search.trim() && (
          <div className="mt-2 divide-y divide-zinc-100 rounded-md border border-zinc-200">
            {filteredStudents.length === 0 ? (
              <p className="px-3 py-2 text-sm text-zinc-400">No matching students.</p>
            ) : (
              filteredStudents.slice(0, 8).map((s) => (
                <Link
                  key={s.id}
                  href={`/fees/students/${s.id}`}
                  className="flex items-center justify-between px-3 py-2 text-sm hover:bg-zinc-50"
                >
                  <span>{s.name_english}</span>
                  <span className="text-zinc-400">{s.admission_number}</span>
                </Link>
              ))
            )}
          </div>
        )}
      </div>

      {error && <p className="mt-4 text-sm text-red-600">{error}</p>}

      {canSeeOfficeView && (
        <div className="mt-6">
          <h2 className="text-sm font-semibold text-zinc-900">Dues &amp; ageing</h2>
          {loading ? (
            <p className="mt-2 text-sm text-zinc-500">Loading…</p>
          ) : (
            <div className="mt-2 overflow-x-auto rounded-md border border-zinc-200 bg-white">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-zinc-200 bg-zinc-50 text-zinc-500">
                  <tr>
                    <th className="px-4 py-2 font-medium">Student</th>
                    <th className="px-4 py-2 font-medium">Outstanding</th>
                    <th className="px-4 py-2 font-medium">Credit balance</th>
                    <th className="px-4 py-2 font-medium">Oldest due</th>
                    <th className="px-4 py-2 font-medium">Days overdue</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-100">
                  {dues.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-4 py-6 text-center text-zinc-400">
                        No outstanding dues.
                      </td>
                    </tr>
                  ) : (
                    dues.map((d) => (
                      <tr key={d.student_id}>
                        <td className="px-4 py-2">
                          <Link href={`/fees/students/${d.student_id}`} className="text-zinc-900 hover:underline">
                            {d.student_name}
                          </Link>
                        </td>
                        <td className="px-4 py-2 font-medium text-zinc-900">{formatPaise(d.outstanding_paise)}</td>
                        <td className="px-4 py-2 text-zinc-500">
                          {d.credit_balance_paise > 0 ? formatPaise(d.credit_balance_paise) : "—"}
                        </td>
                        <td className="px-4 py-2 text-zinc-500">{d.oldest_due_date?.slice(0, 10) ?? "—"}</td>
                        <td className={`px-4 py-2 ${d.days_overdue > 30 ? "font-medium text-red-600" : "text-zinc-500"}`}>
                          {d.days_overdue > 0 ? d.days_overdue : "—"}
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
