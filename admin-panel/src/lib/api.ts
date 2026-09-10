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
