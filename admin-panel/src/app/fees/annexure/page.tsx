"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import { ApiError, annexureExport, formatPaise, listAcademicYears, type AcademicYear, type AnnexureRow } from "@/lib/api";

export default function AnnexurePage() {
  return (
    <RequireAuth>
      <AnnexureContent />
    </RequireAuth>
  );
}

function AnnexureContent() {
  const { accessToken } = useAuth();
  const [years, setYears] = useState<AcademicYear[]>([]);
  const [yearId, setYearId] = useState("");
  const [rows, setRows] = useState<AnnexureRow[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    if (!yearId) return;
    setLoading(true);
    try {
      const r = await annexureExport(accessToken, yearId);
      setRows(r.items);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load the annexure export.");
    } finally {
      setLoading(false);
    }
  }, [accessToken, yearId]);

  useEffect(() => {
    listAcademicYears(accessToken)
      .then((r) => {
        setYears(r.items);
        if (r.items.length > 0) setYearId(r.items[0].id);
      })
      .catch(() => setError("Could not load academic years."));
  }, [accessToken]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  const totals = rows.reduce(
    (acc, r) => ({
      gross: acc.gross + r.gross_paise,
      concession: acc.concession + r.concession_paise,
      net: acc.net + r.net_paise,
      collected: acc.collected + r.collected_paise,
    }),
    { gross: 0, concession: 0, net: 0, collected: 0 },
  );

  function downloadCSV() {
    const header = "Statutory category,Gross,Concession,Net,Collected\n";
    const body = rows
      .map((r) => [r.statutory_category, r.gross_paise / 100, r.concession_paise / 100, r.net_paise / 100, r.collected_paise / 100].join(","))
      .join("\n");
    const blob = new Blob([header + body], { type: "text/csv" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = `fee-annexure-${yearId}.csv`;
    a.click();
    URL.revokeObjectURL(url);
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <Link href="/fees" className="text-sm text-zinc-500 hover:text-zinc-700">
        ← Back to fees
      </Link>
      <h1 className="mt-2 text-lg font-semibold text-zinc-900">Regulatory export</h1>
      <p className="mt-1 text-sm text-zinc-500">
        Fee head amounts by statutory category, for the state fee determination committee filing (PRD 4.4.7).
      </p>

      <div className="mt-4 flex items-center gap-2">
        <select value={yearId} onChange={(e) => setYearId(e.target.value)} className="rounded-md border border-zinc-300 px-3 py-2 text-sm">
          {years.map((y) => (
            <option key={y.id} value={y.id}>
              {y.label}
            </option>
          ))}
        </select>
        <button type="button" onClick={downloadCSV} disabled={rows.length === 0} className="rounded-md border border-zinc-300 px-3 py-2 text-sm text-zinc-700 hover:bg-zinc-100 disabled:opacity-50">
          Download CSV
        </button>
      </div>

      {error && <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}

      {loading ? (
        <p className="mt-6 text-sm text-zinc-500">Loading…</p>
      ) : (
        <div className="mt-4 overflow-x-auto rounded-md border border-zinc-200 bg-white">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-zinc-500">
              <tr>
                <th className="px-3 py-2 font-medium">Statutory category</th>
                <th className="px-3 py-2 font-medium text-right">Gross</th>
                <th className="px-3 py-2 font-medium text-right">Concession</th>
                <th className="px-3 py-2 font-medium text-right">Net</th>
                <th className="px-3 py-2 font-medium text-right">Collected</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-3 py-6 text-center text-zinc-400">
                    No fee data for this year yet.
                  </td>
                </tr>
              ) : (
                rows.map((r) => (
                  <tr key={r.statutory_category}>
                    <td className="px-3 py-2 capitalize">{r.statutory_category}</td>
                    <td className="px-3 py-2 text-right">{formatPaise(r.gross_paise)}</td>
                    <td className="px-3 py-2 text-right text-zinc-500">{formatPaise(r.concession_paise)}</td>
                    <td className="px-3 py-2 text-right font-medium">{formatPaise(r.net_paise)}</td>
                    <td className="px-3 py-2 text-right">{formatPaise(r.collected_paise)}</td>
                  </tr>
                ))
              )}
            </tbody>
            {rows.length > 0 && (
              <tfoot className="border-t border-zinc-200 bg-zinc-50 font-medium">
                <tr>
                  <td className="px-3 py-2">Total</td>
                  <td className="px-3 py-2 text-right">{formatPaise(totals.gross)}</td>
                  <td className="px-3 py-2 text-right">{formatPaise(totals.concession)}</td>
                  <td className="px-3 py-2 text-right">{formatPaise(totals.net)}</td>
                  <td className="px-3 py-2 text-right">{formatPaise(totals.collected)}</td>
                </tr>
              </tfoot>
            )}
          </table>
        </div>
      )}
    </div>
  );
}
