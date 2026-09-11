"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import { ApiError, formatPaise, listReviewRequiredConcessions, resolveConcession, type Concession } from "@/lib/api";

export default function ConcessionsReviewPage() {
  return (
    <RequireAuth>
      <ConcessionsReviewContent />
    </RequireAuth>
  );
}

function ConcessionsReviewContent() {
  const { accessToken } = useAuth();
  const [items, setItems] = useState<Concession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const r = await listReviewRequiredConcessions(accessToken);
      setItems(r.items);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load the review worklist.");
    } finally {
      setLoading(false);
    }
  }, [accessToken]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <Link href="/fees" className="text-sm text-zinc-500 hover:text-zinc-700">
        ← Back to fees
      </Link>
      <h1 className="mt-2 text-lg font-semibold text-zinc-900">Concessions needing review</h1>
      <p className="mt-1 text-sm text-zinc-500">
        These concessions depend on a sibling or staff member who is no longer enrolled/employed. The next fee demand for
        each affected student is blocked until you make an explicit decision.
      </p>

      {error && <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}

      {loading ? (
        <p className="mt-6 text-sm text-zinc-500">Loading…</p>
      ) : items.length === 0 ? (
        <p className="mt-6 text-sm text-zinc-400">Nothing needs review right now.</p>
      ) : (
        <div className="mt-4 space-y-3">
          {items.map((c) => (
            <ReviewCard key={c.id} concession={c} onResolved={load} onError={setError} />
          ))}
        </div>
      )}
    </div>
  );
}

function ReviewCard({
  concession,
  onResolved,
  onError,
}: {
  concession: Concession;
  onResolved: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [note, setNote] = useState("");
  const [pending, setPending] = useState(false);

  async function decide(decision: "active" | "cancelled" | "converted") {
    if (!note.trim()) {
      onError("A note explaining the decision is required.");
      return;
    }
    setPending(true);
    try {
      await resolveConcession(accessToken, concession.id, decision, note);
      onResolved();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not resolve this concession.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="rounded-md border border-amber-300 bg-amber-50 p-4">
      <p className="text-sm font-medium text-zinc-900 capitalize">
        {concession.concession_type} concession —{" "}
        {concession.percentage ? `${concession.percentage}%` : formatPaise(concession.flat_amount_paise ?? 0)}
      </p>
      <p className="mt-1 text-sm text-zinc-600">{concession.reason}</p>
      <input
        placeholder="Note explaining your decision (required)"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        className="mt-2 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
      />
      <div className="mt-2 flex gap-2">
        <button
          disabled={pending}
          onClick={() => decide("active")}
          className="rounded-md bg-emerald-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-emerald-700 disabled:opacity-50"
        >
          Continue
        </button>
        <button
          disabled={pending}
          onClick={() => decide("cancelled")}
          className="rounded-md bg-red-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-red-700 disabled:opacity-50"
        >
          Cancel
        </button>
        <button
          disabled={pending}
          onClick={() => decide("converted")}
          className="rounded-md bg-zinc-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-700 disabled:opacity-50"
        >
          Convert
        </button>
      </div>
    </div>
  );
}
