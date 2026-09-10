"use client";

import { useEffect, useState } from "react";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import {
  ApiError,
  commitStudentImport,
  listAcademicYears,
  previewStudentImport,
  type AcademicYear,
  type ImportCommitResult,
  type ImportPreviewResult,
} from "@/lib/api";

export default function ImportPage() {
  return (
    <RequireAuth>
      <ImportContent />
    </RequireAuth>
  );
}

function ImportContent() {
  const { accessToken } = useAuth();
  const [years, setYears] = useState<AcademicYear[]>([]);
  const [yearId, setYearId] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [preview, setPreview] = useState<ImportPreviewResult | null>(null);
  const [decisions, setDecisions] = useState<Record<string, "link" | "skip">>({});
  const [commitResult, setCommitResult] = useState<ImportCommitResult | null>(null);
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  useEffect(() => {
    listAcademicYears(accessToken)
      .then((r) => {
        setYears(r.items);
        if (r.items.length > 0) setYearId(r.items[0].id);
      })
      .catch(() => setError("Could not load academic years."));
  }, [accessToken]);

  async function runPreview() {
    if (!file) return;
    setPending(true);
    setError("");
    setCommitResult(null);
    try {
      const result = await previewStudentImport(accessToken, file);
      setPreview(result);
      const initialDecisions: Record<string, "link" | "skip"> = {};
      for (const c of result.guardian_clusters) {
        if (c.decision === "manual_review") initialDecisions[c.mobile] = "skip";
      }
      setDecisions(initialDecisions);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not read that file. Is it a valid CSV?");
    } finally {
      setPending(false);
    }
  }

  async function runCommit() {
    if (!file || !yearId) return;
    setPending(true);
    setError("");
    try {
      const result = await commitStudentImport(
        accessToken,
        file,
        yearId,
        Object.entries(decisions).map(([mobile, action]) => ({ mobile, action })),
      );
      setCommitResult(result);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Import failed.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <h1 className="text-lg font-semibold text-zinc-900">Bulk import students</h1>
      <p className="mt-1 text-sm text-zinc-500">
        Upload a CSV, review what will be created, decide on any guardian numbers shared by more than four students,
        then commit.
      </p>

      <div className="mt-6 grid grid-cols-2 gap-4 rounded-md border border-zinc-200 bg-white p-4">
        <div>
          <label className="block text-sm font-medium text-zinc-700">Academic year</label>
          <select
            value={yearId}
            onChange={(e) => setYearId(e.target.value)}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
          >
            {years.length === 0 && <option value="">No academic years yet</option>}
            {years.map((y) => (
              <option key={y.id} value={y.id}>
                {y.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-zinc-700">CSV file</label>
          <input
            type="file"
            accept=".csv"
            onChange={(e) => {
              setFile(e.target.files?.[0] ?? null);
              setPreview(null);
              setCommitResult(null);
            }}
            className="mt-1 w-full text-sm"
          />
        </div>
      </div>

      {error && <p className="mt-4 text-sm text-red-600">{error}</p>}

      <div className="mt-4 flex gap-3">
        <button
          type="button"
          disabled={!file || pending}
          onClick={runPreview}
          className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
        >
          {pending ? "Working…" : "Preview"}
        </button>
        {preview && (
          <button
            type="button"
            disabled={!yearId || pending}
            onClick={runCommit}
            className="rounded-md bg-emerald-700 px-3 py-2 text-sm font-medium text-white hover:bg-emerald-800 disabled:opacity-50"
          >
            {pending ? "Working…" : `Commit (${preview.valid_rows} valid rows)`}
          </button>
        )}
      </div>

      {preview && (
        <div className="mt-6 space-y-6">
          <div className="rounded-md border border-zinc-200 bg-white p-4 text-sm">
            <p>
              <span className="font-medium">{preview.total_rows}</span> rows read,{" "}
              <span className="font-medium">{preview.valid_rows}</span> valid,{" "}
              <span className="font-medium">{preview.errors.length}</span> row errors.
            </p>
          </div>

          {preview.errors.length > 0 && (
            <ErrorTable title="Row errors" errors={preview.errors} />
          )}

          {preview.guardian_clusters.length > 0 && (
            <div className="rounded-md border border-zinc-200 bg-white p-4">
              <h2 className="text-sm font-semibold text-zinc-900">Guardian mobile numbers</h2>
              <p className="mt-1 text-xs text-zinc-500">
                Numbers shared by more than four students need a decision before commit. Placeholder-looking numbers
                (repeated or sequential digits) are rejected automatically and import with no guardian contact.
              </p>
              <ul className="mt-3 divide-y divide-zinc-100">
                {preview.guardian_clusters.map((c) => (
                  <li key={c.mobile} className="flex items-center justify-between gap-4 py-2 text-sm">
                    <div>
                      <span className="font-mono">{c.mobile}</span>
                      <span className="ml-2 text-zinc-500">{c.student_names.join(", ")}</span>
                    </div>
                    <DecisionBadge
                      decision={c.decision}
                      value={decisions[c.mobile]}
                      onChange={(action) => setDecisions((d) => ({ ...d, [c.mobile]: action }))}
                    />
                  </li>
                ))}
              </ul>
            </div>
          )}
        </div>
      )}

      {commitResult && (
        <div className="mt-6 rounded-md border border-emerald-200 bg-emerald-50 p-4 text-sm">
          <p>
            Created <span className="font-medium">{commitResult.students_created}</span> students, linked{" "}
            <span className="font-medium">{commitResult.guardians_linked}</span> guardian relationships.
          </p>
          {commitResult.errors.length > 0 && <ErrorTable title="Rows skipped" errors={commitResult.errors} />}
        </div>
      )}
    </div>
  );
}

function DecisionBadge({
  decision,
  value,
  onChange,
}: {
  decision: string;
  value?: "link" | "skip";
  onChange: (action: "link" | "skip") => void;
}) {
  if (decision === "auto_link") {
    return <span className="rounded-full bg-emerald-100 px-2 py-0.5 text-xs text-emerald-800">Auto-linked</span>;
  }
  if (decision === "rejected") {
    return <span className="rounded-full bg-zinc-100 px-2 py-0.5 text-xs text-zinc-600">No contact (rejected)</span>;
  }
  return (
    <select
      value={value ?? "skip"}
      onChange={(e) => onChange(e.target.value as "link" | "skip")}
      className="rounded-md border border-amber-300 bg-amber-50 px-2 py-1 text-xs text-amber-900"
    >
      <option value="skip">Needs review — skip for now</option>
      <option value="link">Link all as one guardian</option>
    </select>
  );
}

function ErrorTable({ title, errors }: { title: string; errors: { row: number; column: string; reason: string }[] }) {
  return (
    <div className="rounded-md border border-red-200 bg-red-50 p-4">
      <h2 className="text-sm font-semibold text-red-900">{title}</h2>
      <table className="mt-2 w-full text-left text-xs">
        <thead className="text-red-700">
          <tr>
            <th className="pr-4 py-1">Row</th>
            <th className="pr-4 py-1">Column</th>
            <th className="py-1">Reason</th>
          </tr>
        </thead>
        <tbody className="text-red-800">
          {errors.map((e, i) => (
            <tr key={i}>
              <td className="pr-4 py-0.5">{e.row}</td>
              <td className="pr-4 py-0.5">{e.column}</td>
              <td className="py-0.5">{e.reason}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
