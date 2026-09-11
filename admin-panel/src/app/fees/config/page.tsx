"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import {
  ApiError,
  STATUTORY_CATEGORIES,
  createFeeHead,
  createFeeStructure,
  formatPaise,
  listAcademicYears,
  listClasses,
  listFeeHeads,
  listFeeStructures,
  listInstalments,
  rupeesToPaise,
  type AcademicYear,
  type Class,
  type FeeHead,
  type FeeStructure,
  type Instalment,
  type InstalmentInput,
  type StatutoryCategory,
} from "@/lib/api";

export default function FeeConfigPage() {
  return (
    <RequireAuth>
      <FeeConfigContent />
    </RequireAuth>
  );
}

function FeeConfigContent() {
  const { accessToken, activeSchool } = useAuth();
  const [years, setYears] = useState<AcademicYear[]>([]);
  const [yearId, setYearId] = useState("");
  const [heads, setHeads] = useState<FeeHead[]>([]);
  const [classes, setClasses] = useState<Class[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");

  const isCorrespondent = activeSchool?.role === "correspondent";

  const load = useCallback(async () => {
    if (!yearId) return;
    try {
      const [h, c] = await Promise.all([listFeeHeads(accessToken, yearId), listClasses(accessToken, yearId)]);
      setHeads(h.items);
      setClasses(c.items);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load fee configuration.");
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

  if (!isCorrespondent) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-8">
        <p className="text-sm text-zinc-500">Only the correspondent may configure fee heads and structures (PRD role matrix).</p>
        <Link href="/fees" className="mt-2 inline-block text-sm text-zinc-500 underline">
          Back to fees
        </Link>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8">
      <Link href="/fees" className="text-sm text-zinc-500 hover:text-zinc-700">
        ← Back to fees
      </Link>
      <h1 className="mt-2 text-lg font-semibold text-zinc-900">Fee configuration</h1>

      <div className="mt-4">
        <label className="block text-sm font-medium text-zinc-700">Academic year</label>
        <select value={yearId} onChange={(e) => setYearId(e.target.value)} className="mt-1 rounded-md border border-zinc-300 px-3 py-2 text-sm">
          {years.map((y) => (
            <option key={y.id} value={y.id}>
              {y.label}
            </option>
          ))}
        </select>
      </div>

      {notice && <p className="mt-3 rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</p>}
      {error && <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}

      {yearId && (
        <>
          <FeeHeadsSection
            yearId={yearId}
            heads={heads}
            onCreated={() => {
              setNotice("Fee head created.");
              load();
            }}
            onError={setError}
          />
          <FeeStructuresSection
            yearId={yearId}
            classes={classes}
            heads={heads}
            onCreated={() => setNotice("Fee structure saved.")}
            onError={setError}
          />
        </>
      )}
    </div>
  );
}

function FeeHeadsSection({
  yearId,
  heads,
  onCreated,
  onError,
}: {
  yearId: string;
  heads: FeeHead[];
  onCreated: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [name, setName] = useState("");
  const [category, setCategory] = useState<StatutoryCategory>("tuition");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!name) return;
    setPending(true);
    try {
      await createFeeHead(accessToken, { academic_year_id: yearId, name, statutory_category: category });
      setName("");
      onCreated();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not create this fee head.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mt-6">
      <h2 className="text-sm font-semibold text-zinc-900">Fee heads</h2>
      <div className="mt-2 divide-y divide-zinc-100 rounded-md border border-zinc-200 bg-white">
        {heads.length === 0 ? (
          <p className="px-3 py-4 text-center text-sm text-zinc-400">No fee heads yet for this year.</p>
        ) : (
          heads.map((h) => (
            <div key={h.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <span>{h.name}</span>
              <span className="text-zinc-500 capitalize">{h.statutory_category}</span>
            </div>
          ))
        )}
      </div>
      <form onSubmit={onSubmit} className="mt-2 flex items-end gap-2">
        <div className="flex-1">
          <label className="block text-xs font-medium text-zinc-500">Name</label>
          <input value={name} onChange={(e) => setName(e.target.value)} className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm" />
        </div>
        <div>
          <label className="block text-xs font-medium text-zinc-500">Statutory category</label>
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value as StatutoryCategory)}
            className="mt-1 rounded-md border border-zinc-300 px-3 py-2 text-sm capitalize"
          >
            {STATUTORY_CATEGORIES.map((c) => (
              <option key={c} value={c} className="capitalize">
                {c}
              </option>
            ))}
          </select>
        </div>
        <button type="submit" disabled={pending} className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50">
          {pending ? "Adding…" : "Add head"}
        </button>
      </form>
    </div>
  );
}

function FeeStructuresSection({
  yearId,
  classes,
  heads,
  onCreated,
  onError,
}: {
  yearId: string;
  classes: Class[];
  heads: FeeHead[];
  onCreated: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [classId, setClassId] = useState("");
  const [structures, setStructures] = useState<FeeStructure[]>([]);
  const [instalments, setInstalments] = useState<Instalment[]>([]);
  const [rows, setRows] = useState<InstalmentInput[]>([{ fee_head_id: "", label: "", amount_paise: 0, due_date: "" }]);
  const [amountInputs, setAmountInputs] = useState<string[]>([""]);
  const [pending, setPending] = useState(false);

  useEffect(() => {
    if (classes.length > 0 && !classId) setClassId(classes[0].id);
  }, [classes, classId]);

  const loadStructures = useCallback(async () => {
    if (!classId) return;
    const s = await listFeeStructures(accessToken, yearId, classId);
    setStructures(s.items);
    const active = s.items.find((x) => x.is_active);
    if (active) {
      const i = await listInstalments(accessToken, active.id);
      setInstalments(i.items);
    } else {
      setInstalments([]);
    }
  }, [accessToken, yearId, classId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    loadStructures();
  }, [loadStructures]);

  function updateRow(i: number, patch: Partial<InstalmentInput>) {
    setRows((r) => r.map((row, idx) => (idx === i ? { ...row, ...patch } : row)));
  }

  function updateAmount(i: number, value: string) {
    setAmountInputs((a) => a.map((v, idx) => (idx === i ? value : v)));
    updateRow(i, { amount_paise: rupeesToPaise(value) });
  }

  function addRow() {
    setRows((r) => [...r, { fee_head_id: "", label: "", amount_paise: 0, due_date: "" }]);
    setAmountInputs((a) => [...a, ""]);
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const valid = rows.filter((r) => r.fee_head_id && r.label && r.due_date && r.amount_paise > 0);
    if (valid.length === 0) {
      onError("Add at least one instalment with a head, label, amount and due date.");
      return;
    }
    setPending(true);
    try {
      await createFeeStructure(accessToken, { academic_year_id: yearId, class_id: classId, instalments: valid });
      setRows([{ fee_head_id: "", label: "", amount_paise: 0, due_date: "" }]);
      setAmountInputs([""]);
      onCreated();
      loadStructures();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not save this fee structure.");
    } finally {
      setPending(false);
    }
  }

  const activeStructure = structures.find((s) => s.is_active);

  return (
    <div className="mt-8">
      <h2 className="text-sm font-semibold text-zinc-900">Fee structures</h2>
      <div className="mt-2">
        <label className="block text-xs font-medium text-zinc-500">Class</label>
        <select value={classId} onChange={(e) => setClassId(e.target.value)} className="mt-1 rounded-md border border-zinc-300 px-3 py-2 text-sm">
          {classes.map((c) => (
            <option key={c.id} value={c.id}>
              {c.name}
            </option>
          ))}
        </select>
      </div>

      {activeStructure && (
        <div className="mt-3">
          <p className="text-xs text-zinc-500">Active structure: version {activeStructure.version}</p>
          <div className="mt-1 divide-y divide-zinc-100 rounded-md border border-zinc-200 bg-white">
            {instalments.map((i) => (
              <div key={i.id} className="flex items-center justify-between px-3 py-2 text-sm">
                <span>
                  {heads.find((h) => h.id === i.fee_head_id)?.name ?? i.fee_head_id} — {i.label}
                </span>
                <span className="text-zinc-500">
                  {formatPaise(i.amount_paise)} due {i.due_date.slice(0, 10)}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      <form onSubmit={onSubmit} className="mt-4 rounded-md border border-zinc-200 bg-white p-4">
        <p className="text-xs font-medium text-zinc-500">
          {activeStructure ? "Create a new version (does not alter already-issued demands)" : "Create the fee structure"}
        </p>
        <div className="mt-2 space-y-2">
          {rows.map((row, i) => (
            <div key={i} className="grid grid-cols-4 gap-2">
              <select
                value={row.fee_head_id}
                onChange={(e) => updateRow(i, { fee_head_id: e.target.value })}
                className="rounded-md border border-zinc-300 px-2 py-1.5 text-sm"
              >
                <option value="">Fee head…</option>
                {heads.map((h) => (
                  <option key={h.id} value={h.id}>
                    {h.name}
                  </option>
                ))}
              </select>
              <input
                placeholder="Label (e.g. Term 1)"
                value={row.label}
                onChange={(e) => updateRow(i, { label: e.target.value })}
                className="rounded-md border border-zinc-300 px-2 py-1.5 text-sm"
              />
              <input
                placeholder="Amount (₹)"
                value={amountInputs[i]}
                onChange={(e) => updateAmount(i, e.target.value)}
                className="rounded-md border border-zinc-300 px-2 py-1.5 text-sm"
              />
              <input
                type="date"
                value={row.due_date}
                onChange={(e) => updateRow(i, { due_date: e.target.value })}
                className="rounded-md border border-zinc-300 px-2 py-1.5 text-sm"
              />
            </div>
          ))}
        </div>
        <div className="mt-2 flex gap-2">
          <button type="button" onClick={addRow} className="text-sm text-zinc-500 hover:text-zinc-700">
            + Add instalment
          </button>
        </div>
        <button type="submit" disabled={pending} className="mt-3 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50">
          {pending ? "Saving…" : "Save structure"}
        </button>
      </form>
    </div>
  );
}
