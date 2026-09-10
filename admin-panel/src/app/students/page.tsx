"use client";

import { useCallback, useEffect, useState } from "react";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import { ApiError, createStudent, listStudents, type Student } from "@/lib/api";

export default function StudentsPage() {
  return (
    <RequireAuth>
      <StudentsContent />
    </RequireAuth>
  );
}

function StudentsContent() {
  const { accessToken } = useAuth();
  const [students, setStudents] = useState<Student[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [showForm, setShowForm] = useState(false);

  // fetchStudents sets no state before its first await, so calling it directly
  // from the mount effect below never sets state synchronously within the
  // effect -- loading already starts true, so there's nothing to set going in.
  const fetchStudents = useCallback(async () => {
    try {
      const result = await listStudents(accessToken);
      setStudents(result.items);
      setError("");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load students.");
    } finally {
      setLoading(false);
    }
  }, [accessToken]);

  useEffect(() => {
    // Standard fetch-on-mount: the lint rule flags any effect that transitively
    // calls setState, but data fetching on mount is exactly what this effect is
    // for, and fetchStudents already starts loading=true so there's nothing lost
    // by not modelling this as a subscription to an external store.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    fetchStudents();
  }, [fetchStudents]);

  // refresh is for callers outside the mount effect (e.g. after creating a
  // student), where setting loading back to true synchronously is fine -- it's
  // not running inside a useEffect body.
  const refresh = useCallback(() => {
    setLoading(true);
    fetchStudents();
  }, [fetchStudents]);

  return (
    <div className="mx-auto max-w-5xl px-4 py-8">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold text-zinc-900">Students</h1>
        <button
          type="button"
          onClick={() => setShowForm((v) => !v)}
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white hover:bg-zinc-800"
        >
          {showForm ? "Cancel" : "Add student"}
        </button>
      </div>

      {showForm && (
        <NewStudentForm
          onCreated={() => {
            setShowForm(false);
            refresh();
          }}
        />
      )}

      {error && <p className="mt-4 text-sm text-red-600">{error}</p>}
      {loading ? (
        <p className="mt-6 text-sm text-zinc-500">Loading…</p>
      ) : (
        <div className="mt-6 overflow-x-auto rounded-md border border-zinc-200 bg-white">
          <table className="w-full text-left text-sm">
            <thead className="border-b border-zinc-200 bg-zinc-50 text-zinc-500">
              <tr>
                <th className="px-4 py-2 font-medium">Admission #</th>
                <th className="px-4 py-2 font-medium">Name</th>
                <th className="px-4 py-2 font-medium">Gender</th>
                <th className="px-4 py-2 font-medium">Date of birth</th>
                <th className="px-4 py-2 font-medium">RTE</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-100">
              {students.length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-4 py-6 text-center text-zinc-400">
                    No students yet.
                  </td>
                </tr>
              ) : (
                students.map((s) => (
                  <tr key={s.id}>
                    <td className="px-4 py-2">{s.admission_number}</td>
                    <td className="px-4 py-2">{s.name_english}</td>
                    <td className="px-4 py-2">{s.gender}</td>
                    <td className="px-4 py-2">{s.date_of_birth.slice(0, 10)}</td>
                    <td className="px-4 py-2">{s.rte_quota ? "Yes" : "—"}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function NewStudentForm({ onCreated }: { onCreated: () => void }) {
  const { accessToken } = useAuth();
  const [error, setError] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setPending(true);
    setError("");
    const form = new FormData(e.currentTarget);
    try {
      await createStudent(accessToken, {
        admission_number: String(form.get("admission_number")),
        name_english: String(form.get("name_english")),
        name_tamil: String(form.get("name_tamil") || "") || undefined,
        gender: String(form.get("gender")),
        date_of_birth: String(form.get("date_of_birth")),
        admission_date: String(form.get("admission_date")),
        rte_quota: form.get("rte_quota") === "on",
      });
      onCreated();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create student.");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="mt-4 grid grid-cols-2 gap-4 rounded-md border border-zinc-200 bg-white p-4">
      <Field label="Admission number" name="admission_number" required />
      <Field label="Name (English)" name="name_english" required />
      <Field label="Name (Tamil)" name="name_tamil" />
      <div>
        <label className="block text-sm font-medium text-zinc-700">Gender</label>
        <select name="gender" required className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm">
          <option value="">Select…</option>
          <option value="M">Male</option>
          <option value="F">Female</option>
          <option value="Other">Other</option>
        </select>
      </div>
      <Field label="Date of birth" name="date_of_birth" type="date" required />
      <Field label="Admission date" name="admission_date" type="date" required />
      <label className="col-span-2 flex items-center gap-2 text-sm text-zinc-700">
        <input type="checkbox" name="rte_quota" className="rounded border-zinc-300" />
        RTE quota student
      </label>
      {error && <p className="col-span-2 text-sm text-red-600">{error}</p>}
      <button
        type="submit"
        disabled={pending}
        className="col-span-2 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
      >
        {pending ? "Saving…" : "Save student"}
      </button>
    </form>
  );
}

function Field({
  label,
  name,
  type = "text",
  required,
}: {
  label: string;
  name: string;
  type?: string;
  required?: boolean;
}) {
  return (
    <div>
      <label htmlFor={name} className="block text-sm font-medium text-zinc-700">
        {label}
      </label>
      <input
        id={name}
        name={name}
        type={type}
        required={required}
        className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm focus:border-zinc-500 focus:outline-none"
      />
    </div>
  );
}
