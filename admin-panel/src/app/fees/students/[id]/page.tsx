"use client";

import { use, useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { RequireAuth } from "@/components/require-auth";
import { useAuth } from "@/lib/auth-context";
import {
  ApiError,
  ConcessionReviewRequiredError,
  collectPayment,
  createConcession,
  formatPaise,
  generateFeeAssignment,
  getFeeLineItems,
  getStudent,
  listAcademicYears,
  listConcessions,
  paymentsForStudent,
  refundPayment,
  rupeesToPaise,
  setChequeStatus,
  voidPayment,
  type AcademicYear,
  type AllocationInput,
  type ChequeStatus,
  type Concession,
  type ConcessionType,
  type FeeLineItem,
  type Payment,
  type PaymentMode,
  type Student,
} from "@/lib/api";

export default function StudentFeesPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  return (
    <RequireAuth>
      <StudentFeesContent studentId={id} />
    </RequireAuth>
  );
}

function StudentFeesContent({ studentId }: { studentId: string }) {
  const { accessToken, activeSchool } = useAuth();
  const canCollect = activeSchool?.role === "correspondent" || activeSchool?.role === "office_admin";
  const canVoidRefund = activeSchool?.role === "correspondent";

  const [student, setStudent] = useState<Student | null>(null);
  const [years, setYears] = useState<AcademicYear[]>([]);
  const [yearId, setYearId] = useState("");
  const [lineItems, setLineItems] = useState<FeeLineItem[]>([]);
  const [creditBalance, setCreditBalance] = useState(0);
  const [payments, setPayments] = useState<Payment[]>([]);
  const [concessions, setConcessions] = useState<Concession[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [reviewNeeded, setReviewNeeded] = useState<Concession[] | null>(null);
  const [notice, setNotice] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [s, li, ps, cs, ys] = await Promise.all([
        getStudent(accessToken, studentId),
        getFeeLineItems(accessToken, studentId),
        paymentsForStudent(accessToken, studentId),
        listConcessions(accessToken, studentId),
        listAcademicYears(accessToken),
      ]);
      setStudent(s);
      setLineItems(li.items);
      setCreditBalance(li.credit_balance_paise);
      setPayments(ps.items);
      setConcessions(cs.items);
      setYears(ys.items);
      if (ys.items.length > 0 && !yearId) setYearId(ys.items[0].id);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not load this student's fee record.");
    } finally {
      setLoading(false);
    }
    // yearId intentionally excluded -- set once from the load itself, not a dependency that should re-trigger this.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [accessToken, studentId]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    load();
  }, [load]);

  async function onGenerateAssignment() {
    if (!yearId) return;
    setError("");
    setReviewNeeded(null);
    try {
      await generateFeeAssignment(accessToken, studentId, yearId);
      setNotice("Fee demand generated.");
      await load();
    } catch (err) {
      if (err instanceof ConcessionReviewRequiredError) {
        setReviewNeeded(err.concessions);
        return;
      }
      setError(err instanceof ApiError ? err.message : "Could not generate the fee assignment.");
    }
  }

  const pendingItems = lineItems.filter((li) => li.status !== "paid" && li.status !== "waived");

  return (
    <div className="mx-auto max-w-4xl px-4 py-8">
      <Link href="/fees" className="text-sm text-zinc-500 hover:text-zinc-700">
        ← Back to fees
      </Link>

      {loading ? (
        <p className="mt-4 text-sm text-zinc-500">Loading…</p>
      ) : (
        <>
          <div className="mt-2 flex items-baseline justify-between">
            <div>
              <h1 className="text-lg font-semibold text-zinc-900">{student?.name_english}</h1>
              <p className="text-sm text-zinc-500">Admission #{student?.admission_number}</p>
            </div>
            {creditBalance > 0 && (
              <p className="text-sm text-emerald-700">
                Credit balance: <span className="font-medium">{formatPaise(creditBalance)}</span>
              </p>
            )}
          </div>

          {notice && <p className="mt-3 rounded-md bg-emerald-50 px-3 py-2 text-sm text-emerald-700">{notice}</p>}
          {error && <p className="mt-3 rounded-md bg-red-50 px-3 py-2 text-sm text-red-600">{error}</p>}

          {reviewNeeded && (
            <div className="mt-3 rounded-md border border-amber-300 bg-amber-50 p-3 text-sm text-amber-900">
              <p className="font-medium">
                {reviewNeeded.length} concession(s) need office review before this demand can be generated.
              </p>
              <ul className="mt-1 list-inside list-disc">
                {reviewNeeded.map((c) => (
                  <li key={c.id}>
                    {c.concession_type} concession — {c.reason}
                  </li>
                ))}
              </ul>
              <Link href="/fees/concessions" className="mt-2 inline-block font-medium text-amber-900 underline">
                Resolve on the concessions review page
              </Link>
            </div>
          )}

          {lineItems.length === 0 && canCollect && (
            <div className="mt-4 rounded-md border border-zinc-200 bg-white p-4">
              <p className="text-sm text-zinc-700">No fee demand has been generated for this student yet.</p>
              <div className="mt-2 flex items-center gap-2">
                <select
                  value={yearId}
                  onChange={(e) => setYearId(e.target.value)}
                  className="rounded-md border border-zinc-300 px-3 py-2 text-sm"
                >
                  {years.map((y) => (
                    <option key={y.id} value={y.id}>
                      {y.label}
                    </option>
                  ))}
                </select>
                <button
                  type="button"
                  onClick={onGenerateAssignment}
                  className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800"
                >
                  Generate fee demand
                </button>
              </div>
            </div>
          )}

          {lineItems.length > 0 && (
            <div className="mt-6">
              <h2 className="text-sm font-semibold text-zinc-900">Fee line items</h2>
              <div className="mt-2 overflow-x-auto rounded-md border border-zinc-200 bg-white">
                <table className="w-full text-left text-sm">
                  <thead className="border-b border-zinc-200 bg-zinc-50 text-zinc-500">
                    <tr>
                      <th className="px-3 py-2 font-medium">Head</th>
                      <th className="px-3 py-2 font-medium">Instalment</th>
                      <th className="px-3 py-2 font-medium">Due</th>
                      <th className="px-3 py-2 font-medium text-right">Gross</th>
                      <th className="px-3 py-2 font-medium text-right">Concession</th>
                      <th className="px-3 py-2 font-medium text-right">Net</th>
                      <th className="px-3 py-2 font-medium text-right">Paid</th>
                      <th className="px-3 py-2 font-medium">Status</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-zinc-100">
                    {lineItems.map((li) => (
                      <tr key={li.id}>
                        <td className="px-3 py-2">{li.fee_head_name ?? li.fee_head_id}</td>
                        <td className="px-3 py-2">{li.label}</td>
                        <td className="px-3 py-2 text-zinc-500">{li.due_date.slice(0, 10)}</td>
                        <td className="px-3 py-2 text-right">{formatPaise(li.gross_amount_paise)}</td>
                        <td className="px-3 py-2 text-right text-zinc-500">
                          {li.concession_amount_paise > 0 ? `−${formatPaise(li.concession_amount_paise)}` : "—"}
                        </td>
                        <td className="px-3 py-2 text-right font-medium">{formatPaise(li.net_amount_paise)}</td>
                        <td className="px-3 py-2 text-right">{formatPaise(li.paid_amount_paise)}</td>
                        <td className="px-3 py-2">
                          <StatusBadge status={li.status} />
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {canCollect && pendingItems.length > 0 && (
            <CollectPaymentForm
              studentId={studentId}
              pendingItems={pendingItems}
              onCollected={() => {
                setNotice("Payment recorded.");
                load();
              }}
              onError={setError}
            />
          )}

          {creditBalance > 0 && canVoidRefund && (
            <RefundForm
              studentId={studentId}
              creditBalance={creditBalance}
              onDone={() => {
                setNotice("Refund recorded.");
                load();
              }}
              onError={setError}
            />
          )}

          <div className="mt-6">
            <h2 className="text-sm font-semibold text-zinc-900">Payment history</h2>
            <div className="mt-2 overflow-x-auto rounded-md border border-zinc-200 bg-white">
              <table className="w-full text-left text-sm">
                <thead className="border-b border-zinc-200 bg-zinc-50 text-zinc-500">
                  <tr>
                    <th className="px-3 py-2 font-medium">Receipt #</th>
                    <th className="px-3 py-2 font-medium">Date</th>
                    <th className="px-3 py-2 font-medium">Mode</th>
                    <th className="px-3 py-2 font-medium text-right">Amount</th>
                    <th className="px-3 py-2 font-medium">Cheque status</th>
                    <th className="px-3 py-2 font-medium"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-100">
                  {payments.length === 0 ? (
                    <tr>
                      <td colSpan={6} className="px-3 py-6 text-center text-zinc-400">
                        No payments recorded.
                      </td>
                    </tr>
                  ) : (
                    payments.map((p) => (
                      <PaymentRow
                        key={p.id}
                        payment={p}
                        canVoidRefund={canVoidRefund}
                        canCollect={canCollect}
                        onChanged={() => {
                          setNotice("Updated.");
                          load();
                        }}
                        onError={setError}
                      />
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          <ConcessionsPanel
            studentId={studentId}
            concessions={concessions}
            canManage={canCollect}
            onAdded={() => load()}
            onError={setError}
          />
        </>
      )}
    </div>
  );
}

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    pending: "bg-zinc-100 text-zinc-600",
    partially_paid: "bg-amber-100 text-amber-700",
    paid: "bg-emerald-100 text-emerald-700",
    waived: "bg-zinc-100 text-zinc-400",
  };
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-zinc-100 text-zinc-600"}`}>
      {status.replace("_", " ")}
    </span>
  );
}

function PaymentRow({
  payment,
  canVoidRefund,
  canCollect,
  onChanged,
  onError,
}: {
  payment: Payment;
  canVoidRefund: boolean;
  canCollect: boolean;
  onChanged: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [busy, setBusy] = useState(false);

  async function onVoid() {
    const reason = window.prompt("Reason for voiding this receipt (required):");
    if (!reason) return;
    setBusy(true);
    try {
      await voidPayment(accessToken, payment.id, reason);
      onChanged();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not void this payment.");
    } finally {
      setBusy(false);
    }
  }

  async function onChequeTransition(status: ChequeStatus) {
    let addReturnCharge = false;
    let waiverNote = "";
    if (status === "bounced") {
      addReturnCharge = window.confirm("Add the configured cheque return charge to this student's dues?");
      if (!addReturnCharge) waiverNote = window.prompt("Note for waiving the return charge:") ?? "";
    }
    setBusy(true);
    try {
      await setChequeStatus(accessToken, payment.id, status, addReturnCharge, waiverNote);
      onChanged();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not update cheque status.");
    } finally {
      setBusy(false);
    }
  }

  return (
    <tr className={payment.is_void ? "opacity-50" : ""}>
      <td className="px-3 py-2 font-medium">
        #{payment.receipt_number}
        {payment.is_void && <span className="ml-1 text-xs text-red-600">VOID</span>}
      </td>
      <td className="px-3 py-2 text-zinc-500">{payment.collected_at.slice(0, 10)}</td>
      <td className="px-3 py-2 capitalize">{payment.mode}</td>
      <td className="px-3 py-2 text-right">{formatPaise(payment.amount_paise)}</td>
      <td className="px-3 py-2">
        {payment.cheque_status ? (
          <span className="capitalize">{payment.cheque_status}</span>
        ) : (
          <span className="text-zinc-300">—</span>
        )}
      </td>
      <td className="px-3 py-2 text-right">
        {!payment.is_void && (
          <div className="flex justify-end gap-2">
            {canCollect && payment.mode === "cheque" && payment.cheque_status === "received" && (
              <button disabled={busy} onClick={() => onChequeTransition("deposited")} className="text-xs text-zinc-500 hover:text-zinc-800">
                Mark deposited
              </button>
            )}
            {canCollect && payment.mode === "cheque" && payment.cheque_status === "deposited" && (
              <>
                <button disabled={busy} onClick={() => onChequeTransition("cleared")} className="text-xs text-emerald-600 hover:text-emerald-800">
                  Mark cleared
                </button>
                <button disabled={busy} onClick={() => onChequeTransition("bounced")} className="text-xs text-red-600 hover:text-red-800">
                  Mark bounced
                </button>
              </>
            )}
            {canCollect && payment.mode === "cheque" && payment.cheque_status === "received" && (
              <button disabled={busy} onClick={() => onChequeTransition("bounced")} className="text-xs text-red-600 hover:text-red-800">
                Mark bounced
              </button>
            )}
            {canVoidRefund && (
              <button disabled={busy} onClick={onVoid} className="text-xs text-red-600 hover:text-red-800">
                Void
              </button>
            )}
          </div>
        )}
      </td>
    </tr>
  );
}

function CollectPaymentForm({
  studentId,
  pendingItems,
  onCollected,
  onError,
}: {
  studentId: string;
  pendingItems: FeeLineItem[];
  onCollected: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [mode, setMode] = useState<PaymentMode>("cash");
  const [amountRupees, setAmountRupees] = useState("");
  const [selected, setSelected] = useState<Record<string, boolean>>({});
  const [chequeNumber, setChequeNumber] = useState("");
  const [chequeBank, setChequeBank] = useState("");
  const [pending, setPending] = useState(false);

  const amountPaise = rupeesToPaise(amountRupees || "0");
  const selectedItems = pendingItems.filter((li) => selected[li.id]);
  const outstandingSelected = selectedItems.reduce((sum, li) => sum + (li.net_amount_paise - li.paid_amount_paise), 0);
  // Auto-allocate oldest-due-first up to what was selected, capped by what was
  // actually tendered -- the clerk sees and can override the resulting split
  // before submitting (PRD 4.4.3.3: the receipt must show an allocation the
  // parent actually agreed to at the counter, not a server-computed guess).
  const allocatable = Math.min(amountPaise, outstandingSelected);
  const advance = Math.max(0, amountPaise - allocatable);

  function buildAllocations(): AllocationInput[] {
    let remaining = allocatable;
    const out: AllocationInput[] = [];
    for (const li of selectedItems) {
      if (remaining <= 0) break;
      const due = li.net_amount_paise - li.paid_amount_paise;
      const take = Math.min(due, remaining);
      if (take > 0) {
        out.push({ line_item_id: li.id, amount_paise: take });
        remaining -= take;
      }
    }
    return out;
  }

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (amountPaise <= 0) {
      onError("Enter an amount received.");
      return;
    }
    setPending(true);
    try {
      await collectPayment(accessToken, {
        student_id: studentId,
        mode,
        amount_paise: amountPaise,
        allocations: buildAllocations(),
        advance_amount_paise: advance,
        cheque_number: mode === "cheque" ? chequeNumber : undefined,
        cheque_bank: mode === "cheque" ? chequeBank : undefined,
      });
      setAmountRupees("");
      setSelected({});
      setChequeNumber("");
      setChequeBank("");
      onCollected();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not record this payment.");
    } finally {
      setPending(false);
    }
  }

  return (
    <form onSubmit={onSubmit} className="mt-6 rounded-md border border-zinc-200 bg-white p-4">
      <h2 className="text-sm font-semibold text-zinc-900">Collect a payment</h2>

      <div className="mt-3">
        <p className="text-xs font-medium text-zinc-500">Apply to</p>
        <div className="mt-1 divide-y divide-zinc-100 rounded-md border border-zinc-200">
          {pendingItems.map((li) => (
            <label key={li.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <span className="flex items-center gap-2">
                <input
                  type="checkbox"
                  checked={!!selected[li.id]}
                  onChange={(e) => setSelected((s) => ({ ...s, [li.id]: e.target.checked }))}
                  className="rounded border-zinc-300"
                />
                {li.fee_head_name ?? li.fee_head_id} — {li.label}
              </span>
              <span className="text-zinc-500">{formatPaise(li.net_amount_paise - li.paid_amount_paise)} due</span>
            </label>
          ))}
        </div>
      </div>

      <div className="mt-3 grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-zinc-700">Mode</label>
          <select
            value={mode}
            onChange={(e) => setMode(e.target.value as PaymentMode)}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
          >
            <option value="cash">Cash</option>
            <option value="cheque">Cheque</option>
            <option value="upi">UPI</option>
            <option value="card">Card</option>
          </select>
        </div>
        <div>
          <label className="block text-sm font-medium text-zinc-700">Amount received (₹)</label>
          <input
            type="number"
            step="0.01"
            min="0"
            value={amountRupees}
            onChange={(e) => setAmountRupees(e.target.value)}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
          />
        </div>
        {mode === "cheque" && (
          <>
            <div>
              <label className="block text-sm font-medium text-zinc-700">Cheque number</label>
              <input
                value={chequeNumber}
                onChange={(e) => setChequeNumber(e.target.value)}
                className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-zinc-700">Bank</label>
              <input
                value={chequeBank}
                onChange={(e) => setChequeBank(e.target.value)}
                className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
              />
            </div>
          </>
        )}
      </div>

      {amountPaise > 0 && (
        <p className="mt-3 text-sm text-zinc-600">
          Allocated to dues: <span className="font-medium">{formatPaise(allocatable)}</span>
          {advance > 0 && (
            <>
              {" "}
              · Held as credit/advance: <span className="font-medium text-emerald-700">{formatPaise(advance)}</span>
            </>
          )}
        </p>
      )}

      <button
        type="submit"
        disabled={pending}
        className="mt-3 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50"
      >
        {pending ? "Recording…" : "Record payment & issue receipt"}
      </button>
    </form>
  );
}

function RefundForm({
  studentId,
  creditBalance,
  onDone,
  onError,
}: {
  studentId: string;
  creditBalance: number;
  onDone: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [open, setOpen] = useState(false);
  const [amountRupees, setAmountRupees] = useState("");
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    const amount = rupeesToPaise(amountRupees);
    if (amount <= 0 || amount > creditBalance || !reason) {
      onError("Enter a valid refund amount (within the credit balance) and a reason.");
      return;
    }
    setPending(true);
    try {
      await refundPayment(accessToken, { student_id: studentId, amount_paise: amount, reason });
      setAmountRupees("");
      setReason("");
      setOpen(false);
      onDone();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not process this refund.");
    } finally {
      setPending(false);
    }
  }

  if (!open) {
    return (
      <button type="button" onClick={() => setOpen(true)} className="mt-4 text-sm text-zinc-500 underline hover:text-zinc-700">
        Refund from credit balance
      </button>
    );
  }

  return (
    <form onSubmit={onSubmit} className="mt-4 rounded-md border border-zinc-200 bg-white p-4">
      <h2 className="text-sm font-semibold text-zinc-900">Refund from credit balance</h2>
      <p className="mt-1 text-xs text-zinc-500">Available: {formatPaise(creditBalance)}</p>
      <div className="mt-2 grid grid-cols-2 gap-4">
        <div>
          <label className="block text-sm font-medium text-zinc-700">Amount (₹)</label>
          <input
            type="number"
            step="0.01"
            min="0"
            value={amountRupees}
            onChange={(e) => setAmountRupees(e.target.value)}
            className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
          />
        </div>
        <div>
          <label className="block text-sm font-medium text-zinc-700">Reason</label>
          <input value={reason} onChange={(e) => setReason(e.target.value)} className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm" />
        </div>
      </div>
      <div className="mt-3 flex gap-2">
        <button type="submit" disabled={pending} className="rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50">
          {pending ? "Processing…" : "Confirm refund"}
        </button>
        <button type="button" onClick={() => setOpen(false)} className="rounded-md px-3 py-2 text-sm text-zinc-500 hover:text-zinc-700">
          Cancel
        </button>
      </div>
    </form>
  );
}

function ConcessionsPanel({
  studentId,
  concessions,
  canManage,
  onAdded,
  onError,
}: {
  studentId: string;
  concessions: Concession[];
  canManage: boolean;
  onAdded: () => void;
  onError: (msg: string) => void;
}) {
  const { accessToken } = useAuth();
  const [open, setOpen] = useState(false);
  const [type, setType] = useState<ConcessionType>("merit");
  const [amountKind, setAmountKind] = useState<"percentage" | "flat">("percentage");
  const [amount, setAmount] = useState("");
  const [elderStudentId, setElderStudentId] = useState("");
  const [reason, setReason] = useState("");
  const [pending, setPending] = useState(false);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    if (!reason) {
      onError("A reason is required for the concession.");
      return;
    }
    setPending(true);
    try {
      await createConcession(accessToken, {
        student_id: studentId,
        concession_type: type,
        percentage: amountKind === "percentage" ? Number.parseFloat(amount) : undefined,
        flat_amount_paise: amountKind === "flat" ? rupeesToPaise(amount) : undefined,
        elder_student_id: type === "sibling" ? elderStudentId : undefined,
        reason,
      });
      setAmount("");
      setReason("");
      setElderStudentId("");
      setOpen(false);
      onAdded();
    } catch (err) {
      onError(err instanceof ApiError ? err.message : "Could not create this concession.");
    } finally {
      setPending(false);
    }
  }

  return (
    <div className="mt-6">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold text-zinc-900">Concessions</h2>
        {canManage && (
          <button type="button" onClick={() => setOpen((v) => !v)} className="text-sm text-zinc-500 hover:text-zinc-700">
            {open ? "Cancel" : "Add concession"}
          </button>
        )}
      </div>

      {open && (
        <form onSubmit={onSubmit} className="mt-2 grid grid-cols-2 gap-4 rounded-md border border-zinc-200 bg-white p-4">
          <div>
            <label className="block text-sm font-medium text-zinc-700">Type</label>
            <select value={type} onChange={(e) => setType(e.target.value as ConcessionType)} className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm">
              <option value="merit">Merit</option>
              <option value="sibling">Sibling</option>
              <option value="staff_ward">Staff ward</option>
              <option value="management">Management</option>
              <option value="rte">RTE</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-700">Amount kind</label>
            <select value={amountKind} onChange={(e) => setAmountKind(e.target.value as "percentage" | "flat")} className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm">
              <option value="percentage">Percentage</option>
              <option value="flat">Flat amount (₹)</option>
            </select>
          </div>
          <div>
            <label className="block text-sm font-medium text-zinc-700">{amountKind === "percentage" ? "Percentage" : "Amount (₹)"}</label>
            <input value={amount} onChange={(e) => setAmount(e.target.value)} className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm" />
          </div>
          {type === "sibling" && (
            <div>
              <label className="block text-sm font-medium text-zinc-700">Elder sibling's student ID</label>
              <input
                value={elderStudentId}
                onChange={(e) => setElderStudentId(e.target.value)}
                placeholder="Paste the elder sibling's student UUID"
                className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm"
              />
            </div>
          )}
          <div className="col-span-2">
            <label className="block text-sm font-medium text-zinc-700">Reason</label>
            <input value={reason} onChange={(e) => setReason(e.target.value)} required className="mt-1 w-full rounded-md border border-zinc-300 px-3 py-2 text-sm" />
          </div>
          <button type="submit" disabled={pending} className="col-span-2 rounded-md bg-zinc-900 px-3 py-2 text-sm font-medium text-white hover:bg-zinc-800 disabled:opacity-50">
            {pending ? "Saving…" : "Save concession"}
          </button>
        </form>
      )}

      <div className="mt-2 divide-y divide-zinc-100 rounded-md border border-zinc-200 bg-white">
        {concessions.length === 0 ? (
          <p className="px-3 py-4 text-center text-sm text-zinc-400">No concessions on file.</p>
        ) : (
          concessions.map((c) => (
            <div key={c.id} className="flex items-center justify-between px-3 py-2 text-sm">
              <div>
                <span className="font-medium capitalize">{c.concession_type}</span>{" "}
                <span className="text-zinc-500">
                  {c.percentage ? `${c.percentage}%` : formatPaise(c.flat_amount_paise ?? 0)} — {c.reason}
                </span>
              </div>
              <ConcessionStatusBadge status={c.status} />
            </div>
          ))
        )}
      </div>
    </div>
  );
}

function ConcessionStatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    active: "bg-emerald-100 text-emerald-700",
    review_required: "bg-amber-100 text-amber-700",
    cancelled: "bg-zinc-100 text-zinc-400",
    converted: "bg-zinc-100 text-zinc-400",
  };
  return (
    <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${styles[status] ?? "bg-zinc-100 text-zinc-600"}`}>
      {status.replace("_", " ")}
    </span>
  );
}
