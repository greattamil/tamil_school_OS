// Thin fetch wrapper for the Go backend (PRD 8.5: REST under /api/v1). Kept as one
// small module rather than a generated client -- Phase 1 scope is "admin panel
// scaffolding" (PRD section 9), not a full API SDK.
//
// Token storage note: access/refresh tokens are kept in localStorage via
// AuthProvider (see auth-context.tsx). That is a scaffold-quality decision, not a
// production one -- an XSS bug in any client-rendered page would be able to read
// them. Hardening this (httpOnly cookies via a Next.js route-handler proxy, or a
// BFF pattern) belongs with deepening the admin panel in a later phase, not with
// standing it up.
const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

export class ApiError extends Error {
  status: number;
  code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

export interface SchoolRole {
  school_id: string;
  school_name: string;
  role: string;
}

export interface TokenResponse {
  access_token: string;
  refresh_token: string;
  schools: SchoolRole[];
}

async function parseJsonOrThrow<T>(res: Response): Promise<T> {
  const text = await res.text();
  const body = text ? JSON.parse(text) : {};
  if (!res.ok) {
    throw new ApiError(res.status, body.error ?? "unknown_error", body.message ?? res.statusText);
  }
  return body as T;
}

export async function staffLogin(identifier: string, password: string, deviceId: string): Promise<TokenResponse> {
  const res = await fetch(`${API_BASE_URL}/api/v1/auth/staff/login`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ identifier, password, device_id: deviceId }),
  });
  return parseJsonOrThrow<TokenResponse>(res);
}

export async function selectSchool(refreshToken: string, schoolId: string, deviceId: string): Promise<TokenResponse> {
  const res = await fetch(`${API_BASE_URL}/api/v1/auth/select-school`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ refresh_token: refreshToken, school_id: schoolId, device_id: deviceId }),
  });
  return parseJsonOrThrow<TokenResponse>(res);
}

// authedFetch attaches the bearer token to every request. Callers that receive a
// 401 with error "SESSION_REVOKED" should treat it as a forced logout, mirroring
// the mobile client's wipe-on-revocation behaviour (PRD 6.2) rather than retrying.
export async function authedFetch(accessToken: string, path: string, init?: RequestInit): Promise<Response> {
  return fetch(`${API_BASE_URL}${path}`, {
    ...init,
    headers: {
      ...(init?.headers ?? {}),
      Authorization: `Bearer ${accessToken}`,
    },
  });
}

export async function authedJson<T>(accessToken: string, path: string, init?: RequestInit): Promise<T> {
  const res = await authedFetch(accessToken, path, init);
  return parseJsonOrThrow<T>(res);
}

export interface Student {
  id: string;
  admission_number: string;
  name_english: string;
  name_tamil?: string;
  date_of_birth: string;
  gender: string;
  rte_quota: boolean;
  admission_date: string;
  created_at: string;
}

export async function listStudents(accessToken: string, cursor = ""): Promise<{ items: Student[]; next_cursor: string }> {
  const params = cursor ? `?cursor=${encodeURIComponent(cursor)}` : "";
  return authedJson(accessToken, `/api/v1/students${params}`);
}

export async function getStudent(accessToken: string, studentId: string): Promise<Student> {
  return authedJson(accessToken, `/api/v1/students/${studentId}`);
}

export interface CreateStudentInput {
  admission_number: string;
  name_english: string;
  name_tamil?: string;
  gender: string;
  date_of_birth: string;
  admission_date: string;
  rte_quota: boolean;
}

export async function createStudent(accessToken: string, input: CreateStudentInput): Promise<Student> {
  return authedJson(accessToken, "/api/v1/students", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export interface AcademicYear {
  id: string;
  label: string;
  start_date: string;
  end_date: string;
  state: string;
}

export async function listAcademicYears(accessToken: string): Promise<{ items: AcademicYear[] }> {
  return authedJson(accessToken, "/api/v1/academic-years");
}

export interface RowError {
  row: number;
  column: string;
  reason: string;
}

export interface GuardianClusterPreview {
  mobile: string;
  rows: number[];
  student_names: string[];
  decision: "auto_link" | "manual_review" | "rejected";
}

export interface ImportPreviewResult {
  total_rows: number;
  valid_rows: number;
  errors: RowError[];
  guardian_clusters: GuardianClusterPreview[];
}

export async function previewStudentImport(accessToken: string, file: File): Promise<ImportPreviewResult> {
  const form = new FormData();
  form.append("file", file);
  return authedJson(accessToken, "/api/v1/imports/students/preview", { method: "POST", body: form });
}

export interface ImportCommitResult {
  students_created: number;
  guardians_linked: number;
  errors: RowError[];
}

export async function commitStudentImport(
  accessToken: string,
  file: File,
  academicYearId: string,
  guardianDecisions: { mobile: string; action: "link" | "skip" }[],
): Promise<ImportCommitResult> {
  const form = new FormData();
  form.append("file", file);
  form.append("guardian_decisions", JSON.stringify(guardianDecisions));
  return authedJson(accessToken, `/api/v1/imports/students/commit?academic_year_id=${academicYearId}`, {
    method: "POST",
    body: form,
  });
}

// ---------------------------------------------------------------------------
// Fees (Phase 3, PRD 4.4). Types mirror backend/internal/fees exactly -- see
// models_phase3.go / models_drawer.go / repository_regulatory.go for the Go
// side of each of these.

export type StatutoryCategory =
  | "tuition"
  | "special"
  | "laboratory"
  | "library"
  | "computer"
  | "development"
  | "transport"
  | "examination"
  | "other";

export const STATUTORY_CATEGORIES: StatutoryCategory[] = [
  "tuition",
  "special",
  "laboratory",
  "library",
  "computer",
  "development",
  "transport",
  "examination",
  "other",
];

export interface FeeHead {
  id: string;
  academic_year_id: string;
  name: string;
  statutory_category: StatutoryCategory;
  created_at: string;
}

export async function listFeeHeads(accessToken: string, academicYearId: string): Promise<{ items: FeeHead[] }> {
  return authedJson(accessToken, `/api/v1/fees/heads?academic_year_id=${academicYearId}`);
}

export async function createFeeHead(
  accessToken: string,
  input: { academic_year_id: string; name: string; statutory_category: StatutoryCategory },
): Promise<FeeHead> {
  return authedJson(accessToken, "/api/v1/fees/heads", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export interface Class {
  id: string;
  academic_year_id: string;
  name: string;
  sequence: number;
}

export async function listClasses(accessToken: string, academicYearId: string): Promise<{ items: Class[] }> {
  return authedJson(accessToken, `/api/v1/classes?academic_year_id=${academicYearId}`);
}

export interface FeeStructure {
  id: string;
  academic_year_id: string;
  class_id: string;
  version: number;
  is_active: boolean;
  created_at: string;
}

export interface InstalmentInput {
  fee_head_id: string;
  label: string;
  amount_paise: number;
  due_date: string;
}

export interface Instalment extends InstalmentInput {
  id: string;
  structure_id: string;
}

export async function listFeeStructures(
  accessToken: string,
  academicYearId: string,
  classId: string,
): Promise<{ items: FeeStructure[] }> {
  return authedJson(accessToken, `/api/v1/fees/structures?academic_year_id=${academicYearId}&class_id=${classId}`);
}

export async function createFeeStructure(
  accessToken: string,
  input: { academic_year_id: string; class_id: string; instalments: InstalmentInput[] },
): Promise<FeeStructure> {
  return authedJson(accessToken, "/api/v1/fees/structures", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export async function listInstalments(accessToken: string, structureId: string): Promise<{ items: Instalment[] }> {
  return authedJson(accessToken, `/api/v1/fees/structures/${structureId}/instalments`);
}

export type ConcessionType = "sibling" | "staff_ward" | "merit" | "management" | "rte";
export type ConcessionStatus = "active" | "review_required" | "cancelled" | "converted";

export interface Concession {
  id: string;
  student_id: string;
  concession_type: ConcessionType;
  fee_head_id?: string;
  percentage?: number;
  flat_amount_paise?: number;
  elder_student_id?: string;
  dependent_staff_id?: string;
  status: ConcessionStatus;
  approver_id: string;
  reason: string;
  resolution_note?: string;
  created_at: string;
}

export async function listConcessions(accessToken: string, studentId: string): Promise<{ items: Concession[] }> {
  return authedJson(accessToken, `/api/v1/students/${studentId}/concessions`);
}

export async function createConcession(
  accessToken: string,
  input: {
    student_id: string;
    concession_type: ConcessionType;
    fee_head_id?: string;
    percentage?: number;
    flat_amount_paise?: number;
    elder_student_id?: string;
    dependent_staff_id?: string;
    reason: string;
  },
): Promise<Concession> {
  return authedJson(accessToken, "/api/v1/fees/concessions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export async function listReviewRequiredConcessions(accessToken: string): Promise<{ items: Concession[] }> {
  return authedJson(accessToken, "/api/v1/fees/concessions/review-required");
}

export async function resolveConcession(
  accessToken: string,
  concessionId: string,
  decision: "active" | "cancelled" | "converted",
  note: string,
): Promise<void> {
  await authedJson(accessToken, `/api/v1/fees/concessions/${concessionId}/resolve`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ decision, note }),
  });
}

export type LineItemStatus = "pending" | "partially_paid" | "paid" | "waived";

export interface FeeLineItem {
  id: string;
  assignment_id: string;
  student_id: string;
  fee_head_id: string;
  fee_head_name?: string;
  label: string;
  due_date: string;
  gross_amount_paise: number;
  concession_amount_paise: number;
  net_amount_paise: number;
  paid_amount_paise: number;
  status: LineItemStatus;
}

export interface FeeAssignment {
  id: string;
  student_id: string;
  academic_year_id: string;
  structure_id?: string;
  is_rte: boolean;
  created_at: string;
}

export class ConcessionReviewRequiredError extends Error {
  concessions: Concession[];
  constructor(concessions: Concession[], message: string) {
    super(message);
    this.concessions = concessions;
  }
}

export async function generateFeeAssignment(
  accessToken: string,
  studentId: string,
  academicYearId: string,
): Promise<{ assignment: FeeAssignment; line_items: FeeLineItem[] }> {
  const res = await authedFetch(accessToken, `/api/v1/students/${studentId}/fee-assignment`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ academic_year_id: academicYearId }),
  });
  const text = await res.text();
  const body = text ? JSON.parse(text) : {};
  if (res.status === 409 && body.error === "concession_review_required") {
    throw new ConcessionReviewRequiredError(body.concessions ?? [], body.message ?? "Concession review required");
  }
  if (!res.ok) {
    throw new ApiError(res.status, body.error ?? "unknown_error", body.message ?? res.statusText);
  }
  return body;
}

export async function getFeeLineItems(
  accessToken: string,
  studentId: string,
  academicYearId?: string,
): Promise<{ items: FeeLineItem[]; credit_balance_paise: number }> {
  const q = academicYearId ? `?academic_year_id=${academicYearId}` : "";
  return authedJson(accessToken, `/api/v1/students/${studentId}/fee-line-items${q}`);
}

export type PaymentMode = "cash" | "cheque" | "upi" | "card";
export type ChequeStatus = "received" | "deposited" | "cleared" | "bounced";

export interface Payment {
  id: string;
  student_id: string;
  receipt_number: number;
  mode: PaymentMode;
  amount_paise: number;
  allocated_amount_paise: number;
  advance_amount_paise: number;
  collected_by: string;
  collected_at: string;
  cheque_number?: string;
  cheque_bank?: string;
  cheque_status?: ChequeStatus;
  is_void: boolean;
  void_reason?: string;
}

export interface AllocationInput {
  line_item_id: string;
  amount_paise: number;
}

export async function collectPayment(
  accessToken: string,
  input: {
    student_id: string;
    mode: PaymentMode;
    amount_paise: number;
    allocations: AllocationInput[];
    advance_amount_paise: number;
    cheque_number?: string;
    cheque_bank?: string;
  },
): Promise<Payment> {
  return authedJson(accessToken, "/api/v1/fees/payments", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export async function voidPayment(accessToken: string, paymentId: string, reason: string): Promise<void> {
  await authedJson(accessToken, `/api/v1/fees/payments/${paymentId}/void`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ reason }),
  });
}

export async function setChequeStatus(
  accessToken: string,
  paymentId: string,
  status: ChequeStatus,
  addReturnCharge: boolean,
  waiverNote: string,
): Promise<void> {
  await authedJson(accessToken, `/api/v1/fees/payments/${paymentId}/cheque-status`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status, add_return_charge: addReturnCharge, waiver_note: waiverNote }),
  });
}

export async function paymentsForStudent(accessToken: string, studentId: string): Promise<{ items: Payment[] }> {
  return authedJson(accessToken, `/api/v1/students/${studentId}/payments`);
}

export async function refundPayment(
  accessToken: string,
  input: { payment_id?: string; student_id: string; amount_paise: number; reason: string },
): Promise<{ refund_id: string }> {
  return authedJson(accessToken, "/api/v1/fees/refunds", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
}

export interface DuesRow {
  student_id: string;
  student_name: string;
  outstanding_paise: number;
  credit_balance_paise: number;
  oldest_due_date?: string;
  days_overdue: number;
}

export async function duesAgeing(accessToken: string): Promise<{ items: DuesRow[] }> {
  return authedJson(accessToken, "/api/v1/fees/dues-ageing");
}

export interface Denomination {
  note_500: number;
  note_200: number;
  note_100: number;
  note_50: number;
  note_20: number;
  note_10: number;
  coins_paise: number;
}

export type DrawerStatus = "open" | "closed";

export interface CashDrawerClosing {
  id: string;
  collector_id: string;
  business_date: string;
  status: DrawerStatus;
  denomination_breakdown?: Denomination;
  counted_total_paise?: number;
  expected_total_paise?: number;
  variance_paise?: number;
  recount_number: number;
  variance_explanation?: string;
  closed_at?: string;
}

export async function submitCashCount(
  accessToken: string,
  businessDate: string,
  denomination: Denomination,
  explanation: string,
): Promise<CashDrawerClosing> {
  return authedJson(accessToken, "/api/v1/fees/cash-drawer/count", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ business_date: businessDate, denomination, explanation }),
  });
}

export async function closeCashDrawer(accessToken: string, closingId: string): Promise<CashDrawerClosing> {
  return authedJson(accessToken, `/api/v1/fees/cash-drawer/${closingId}/close`, { method: "POST" });
}

export async function listOpenDrawers(accessToken: string): Promise<{ items: CashDrawerClosing[] }> {
  return authedJson(accessToken, "/api/v1/fees/cash-drawer/open");
}

export interface AnnexureRow {
  statutory_category: StatutoryCategory;
  gross_paise: number;
  concession_paise: number;
  net_paise: number;
  collected_paise: number;
}

export async function annexureExport(accessToken: string, academicYearId: string): Promise<{ items: AnnexureRow[] }> {
  return authedJson(accessToken, `/api/v1/fees/annexure?academic_year_id=${academicYearId}`);
}

export function formatPaise(paise: number): string {
  const rupees = paise / 100;
  return `₹${rupees.toLocaleString("en-IN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function rupeesToPaise(rupees: string): number {
  const v = Number.parseFloat(rupees);
  return Number.isFinite(v) ? Math.round(v * 100) : 0;
}
