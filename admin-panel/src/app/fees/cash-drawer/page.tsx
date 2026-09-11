"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import {
  ApiError,
  closeCashDrawer,
  formatPaise,
  listOpenDrawers,
  submitCashCount,
  type CashDrawerClosing,
  type Denomination,
} from "@/lib/api";

const EMPTY_DENOMINATION: Denomination = {
  note_500: 0,
  note_200: 0,
  note_100: 0,
  note_50: 0,
  note_20: 0,
  note_10: 0,
  coins_paise: 0,
};

function todayISO(): string {
  return new Date().toISOString().slice(0, 10);
}

export default function CashDrawerPage() {
  return (
    <RequireAuth>
      <CashDrawerContent />
    </RequireAuth>
  );
}

function CashDrawerContent() {
  const { accessToken } = useAuth();
  const [businessDate, setBusinessDate] = useState(todayISO());
  const [denomination, setDenomination] = useState<Denomination>(EMPTY_DENOMINATION);
  const [explanation, setExplanation] = useState("");
  const [result, setResult] = useState<CashDrawerClosing | null>(null);
  // Tracked separately from `result` (which is cleared by onRecount to go
  // back into blind-entry mode) so the "attempt N of 3" label stays correct
  // across a recount instead of resetting to "attempt 1" every time.
  const [lastRecountNumber, setLastRecountNumber] = useState(0);
  const [openDrawers, setOpenDrawers] = useState<CashDrawerClosing[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState(false);

  const loadOpen = useCallback(async () => {
    try {
      const r = await listOpenDrawers(accessToken);
      setOpenDrawers(r.items);
    } catch {
      // Non-fatal -- the counting flow itself still works without this list.
    }
  }, [accessToken]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadOpen();
  }, [loadOpen]);

  const tolerance = 500; // UI hint only -- the server is the actual authority on tolerance
  const varianceBeyondTolerance = result?.variance_paise != null && Math.abs(result.variance_paise) > tolerance;
  const attemptsLeft = 3 - lastRecountNumber;
  const needsExplanationNow = varianceBeyondTolerance && lastRecountNumber >= 3;

  async function onSubmitCount() {
    setError("");
    setPending(true);
    try {
      const r = await submitCashCount(accessToken, businessDate, denomination, explanation);
      setResult(r);
      setLastRecountNumber(r.recount_number);
      setNotice("");
      loadOpen();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not submit this count.");
    } finally {
      setPending(false);
    }
  }

  function onRecount() {
    setResult(null);
    setError("");
    setDenomination(EMPTY_DENOMINATION);
    // Explanation is deliberately kept -- if the clerk already knows why (e.g.
    // "extra float added this morning"), they shouldn't have to retype it on
    // the next attempt just because the count itself needs redoing.
  }

  async function onClose() {
    if (!result) return;
    setPending(true);
    setError("");
    try {
      await closeCashDrawer(accessToken, result.id);
      setNotice(`Drawer closed for ${businessDate}.`);
      setResult(null);
      setLastRecountNumber(0);
      setDenomination(EMPTY_DENOMINATION);
      setExplanation("");
      loadOpen();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not close this drawer.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl px-4 py-8">
      <Link href="/fees" className="text-sm text-zinc-500 hover:text-zinc-700">
        ← Back to fees
      </Link>
      <h1 className="mt-2 text-lg font-semibold text-zinc-900">Day-end cash drawer closing</h1>

      {openDrawers.length > 0 && (
        <div className="mt-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
          <p className="font-medium">{openDrawers.length} unclosed drawer(s) from prior sessions:</p>
          <ul className="mt-1 list-inside list-disc">
            {openDrawers.map((d) => (
              <li key={d.id}>
                {d.business_date.slice(0, 10)}
                {d.counted_total_paise != null ? ` — last count ${formatPaise(d.counted_total_paise)}` : " — not yet counted"}
              </li>
            ))}
          </ul>
        </div>
      )}

      {notice && <p className="mt-3 rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</p>}
      {error && <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}

      <div className="mt-4 rounded-md border border-zinc-200 bg-white p-4">
        <label className="block text-sm font-medium text-zinc-700">Business date</label>
        <input
          type="date"
          value={businessDate}
          onChange={(e) => setBusinessDate(e.target.value)}
          disabled={!!result}
          className="mt-1 rounded-md border border-zinc-300 px-3 py-2 text-sm disabled:bg-zinc-50"
        />

        <p className="mt-4 text-sm font-medium text-zinc-700">
          Count the drawer and enter the breakdown. The expected total is not shown until after you submit — count first.
        </p>
        <div className="mt-2 grid grid-cols-3 gap-3">
          <DenominationField label="₹500 notes" value={denomination.note_500} onChange={(v) => setDenomination((d) => ({ ...d, note_500: v }))} disabled={!!result} />
          <DenominationField label="₹200 notes" value={denomination.note_200} onChange={(v) => setDenomination((d) => ({ ...d, note_200: v }))} disabled={!!result} />
          <DenominationField label="₹100 notes" value={denomination.note_100} onChange={(v) => setDenomination((d) => ({ ...d, note_100: v }))} disabled={!!result} />
          <DenominationField label="₹50 notes" value={denomination.note_50} onChange={(v) => setDenomination((d) => ({ ...d, note_50: v }))} disabled={!!result} />
          <DenominationField label="₹20 notes" value={denomination.note_20} onChange={(v) => setDenomination((d) => ({ ...d, note_20: v }))} disabled={!!result} />
          <DenominationField label="₹10 notes" value={denomination.note_10} onChange={(v) => setDenomination((d) => ({ ...d, note_10: v }))} disabled={!!result} />
        </div>
        <div className="mt-2">
          <label className="block text-xs font-medium text-zinc-500">Coins (₹)</label>
          <input
            type="number"
            min="0"
            step="0.01"
            disabled={!!result}
            value={denomination.coins_paise ? denomination.coins_paise / 100 : ""}
            onChange={(e) => setDenomination((d) => ({ ...d, coins_paise: Math.round(Number.parseFloat(e.target.value || "0") * 100) }))}
            className="mt-1 w-32 rounded-md border border-zinc-300 px-2 py-1.5 text-sm disabled:bg-zinc-50"
          />
        </div>

        <div className="mt-2">
          <label className="block text-xs font-medium text-zinc-500">
            Variance explanation {needsExplanationNow ? "(required now — the two allowed recounts are used up)" : "(only needed if this is your third count and it doesn't match)"}
          </label>
          <input
            value={explanation}
            onChange={(e) => setExplanation(e.target.value)}
            disabled={!!result}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm disabled:bg-zinc-50"
          />
        </div>

        <p className="mt-3 text-sm text-zinc-600">
          Counted total: <span className="font-medium">{formatPaise(counted(denomination))}</span>
        </p>

        {!result ? (
          <button
            type="button"
            onClick={onSubmitCount}
            disabled={pending}
            className="mt-4 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
          >
            {pending ? "Submitting…" : lastRecountNumber > 0 ? `Submit recount (attempt ${lastRecountNumber + 1} of 3)` : "Submit count"}
          </button>
        ) : (
          <div className="mt-4 rounded-md border border-zinc-300 bg-zinc-50 p-3">
            <p className="text-sm">
              Expected: <span className="font-medium">{formatPaise(result.expected_total_paise ?? 0)}</span>
            </p>
            <p className="text-sm">
              Counted: <span className="font-medium">{formatPaise(result.counted_total_paise ?? 0)}</span>
            </p>
            <p className={`text-sm font-medium ${varianceBeyondTolerance ? "text-red-600" : "text-emerald-700"}`}>
              Variance: {formatPaise(result.variance_paise ?? 0)}
              {varianceBeyondTolerance ? " (beyond tolerance)" : " (within tolerance)"}
            </p>
            <p className="mt-1 text-xs text-zinc-500">Attempt {result.recount_number} of 3</p>

            <div className="mt-3 flex gap-2">
              {attemptsLeft > 0 && varianceBeyondTolerance && (
                <button type="button" onClick={onRecount} className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm text-zinc-700 hover:bg-zinc-100">
                  Recount ({attemptsLeft} attempt(s) left)
                </button>
              )}
              <button
                type="button"
                onClick={onClose}
                disabled={pending}
                className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
              >
                {pending ? "Closing…" : "Close drawer"}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function counted(d: Denomination): number {
  return d.note_500 * 50000 + d.note_200 * 20000 + d.note_100 * 10000 + d.note_50 * 5000 + d.note_20 * 2000 + d.note_10 * 1000 + d.coins_paise;
}

function DenominationField({
  label,
  value,
  onChange,
  disabled,
}: {
  label: string;
  value: number;
  onChange: (v: number) => void;
  disabled: boolean;
}) {
  return (
    <div>
      <label className="block text-xs font-medium text-zinc-500">{label}</label>
      <input
        type="number"
        min="0"
        disabled={disabled}
        value={value || ""}
        onChange={(e) => onChange(Number.parseInt(e.target.value || "0", 10))}
        className="mt-1 w-full rounded-md border border-zinc-300 px-2 py-1.5 text-sm disabled:bg-zinc-50"
      />
    </div>
  );
}
