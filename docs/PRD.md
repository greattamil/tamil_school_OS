# Product Requirements Document
## School Management System for Tamil Nadu Private Matriculation Schools

**Version:** 1.0
**Date:** September 2026
**Owner:** Solo developer
**Status:** Draft for first design-partner school

---

## 1. Overview

### 1.1 Product summary

A mobile-first school management system for private matriculation schools in Tamil Nadu, covering student records, attendance, fee collection, examinations, report cards and parent communication.

The product is delivered as an Android application for teachers and parents, plus a responsive web administration panel for office staff. The backend is a multi-tenant Go service so that additional schools can be onboarded without redeployment.

### 1.2 Problem statement

Mid-sized private matriculation schools in Tamil Nadu (roughly 300 to 1,500 students) typically operate on paper registers, spreadsheets and WhatsApp groups. The consequences are:

- Fee dues are tracked manually, so follow-up is slow and outstanding amounts stay high.
- Attendance registers are paper-based, making pattern analysis and parent notification impossible.
- Report card preparation consumes weeks of teacher time each term.
- Parent communication is ad hoc, with no delivery record and no ability to target a section or class.
- Office staff answer repetitive phone calls about dues, marks and holidays.

Existing national ERP products are either expensive relative to school revenue, poorly localised for Tamil, weakly matched to matriculation report card formats, or all three.

### 1.3 Product principles

1. **The school's daily reality wins over feature completeness.** A working attendance screen used by every teacher beats twelve modules used by nobody.
2. **Offline is the default assumption, not the edge case.** Indian school buildings have poor indoor connectivity.
3. **Money is never optimistic.** Fee transactions are online-only, synchronous, and audited.
4. **Tenant isolation is enforced by the database, not by developer discipline.**
5. **Tamil for parents, English for staff.**
6. **Every screen a teacher uses must work in under ten seconds of interaction.**

### 1.4 Non-goals for v1

Explicitly out of scope, to be reconsidered only when a paying school requests them:

- Learning management, content delivery or online classes
- Transport and GPS bus tracking
- Library, hostel and inventory management
- Payroll and HR
- Biometric device integration
- iOS application
- Automated timetable generation (display only in v1)
- AI features of any kind

---

## 2. Users and roles

### 2.1 Personas

**Correspondent / Principal (buyer).** Cares about fee collection rates, admission numbers and the school's image with parents. Uses the system weekly for dashboards, not daily. This person signs the cheque, so the fee dashboard is the most important screen in the product.

**Office administrator (power user).** Handles admissions, fee receipts, certificates and record-keeping. Works on a desktop for several hours a day. Currently lives in Excel and a fee register. Their trust is earned by making bulk work faster, not by making it prettier.

**Class teacher.** Marks attendance daily, enters marks each exam cycle, sends notices to their section. Uses a personal Android phone, often mid-range, in a classroom with weak signal. Will abandon the app permanently if it loses a period's attendance.

**Subject teacher.** Enters marks for their subject across several sections. Lower-frequency use than class teachers.

**Parent.** Checks attendance, marks, fee dues and school notices. Highly variable device quality and digital literacy. Many prefer Tamil. Some have multiple children in the school. Some households have separated parents needing independent access.

### 2.2 Role and permission matrix

| Capability | Correspondent | Office admin | Class teacher | Subject teacher | Parent |
|---|---|---|---|---|---|
| View school-wide dashboards | Yes | Yes | No | No | No |
| Create/edit student records | Yes | Yes | No | No | No |
| Create/edit staff records | Yes | Yes | No | No | No |
| Mark attendance | Yes | Yes | Own section | No | No |
| View attendance | All | All | Own section | Own sections | Own children |
| Enter marks | Yes | Yes | Own section | Own subjects | No |
| Publish report cards | Yes | Yes | No | No | No |
| View report cards | All | All | Own section | No | Own children |
| Record fee payment | Yes | Yes | No | No | No |
| Void/refund a payment | Yes | No | No | No | No |
| View fee dues | All | All | Own section (summary) | No | Own children |
| Send notice to whole school | Yes | Yes | No | No | No |
| Send notice to own section | Yes | Yes | Yes | No | No |
| Issue certificates (TC, bonafide) | Yes | Yes | No | No | No |
| View audit log | Yes | No | No | No | No |
| Configure fee structure | Yes | No | No | No | No |
| Configure report card format | Yes | Yes | No | No | No |

Roles are per school and per academic year. A user may hold multiple roles — a teacher who is also a parent at the same school is an expected case, not an exception.

---

## 3. Domain model

### 3.1 Core concepts

**Academic year** is the spine of the entire system. Almost every record is scoped to an academic year. A student is not "in Class 6B" — a student is in Class 6B *for 2026-27*. This must be designed in from the first migration; retrofitting it is the single most expensive mistake possible in school software, because historical report cards and fee records must remain accurate after promotion.

**An academic year has a lifecycle, not just an identity.** Report cards, transfer certificates and fee records are legal instruments, and a well-meaning admin correcting something in October 2027 must not be able to silently alter the 2026-27 record that a TC was issued from. Three states:

| State | Marks and attendance | Fees | Certificates |
|---|---|---|---|
| `active` | Read/write per role | Read/write | Issued |
| `soft_closing` | Locked | Arrears collection permitted | Issued |
| `locked` | Read-only | Read-only | Reissue of existing only |

- `soft_closing` begins at term end and runs for a configurable buffer (default 30 days), covering the real situation where marks are final but a family is still clearing last year's dues and a TC is still being requested.
- `locked` is enforced in two places: the application layer refuses writes, and an RLS policy rejects `UPDATE` and `DELETE` against rows scoped to a locked year. Application-only enforcement is not sufficient, because a future migration script or an admin panel bulk action can bypass it.
- **Reopening a locked year requires correspondent credentials with re-authentication**, carries a mandatory reason, is time-boxed, and writes a prominent audit entry. It should feel deliberate and slightly uncomfortable, because it is.
- Transitions are recorded with who made them and when.

**School (tenant).** Every table carrying school data has a `school_id`. Isolation is enforced by PostgreSQL row-level security.

**Student.** A person with a permanent admission number, stable across academic years. Enrollment (class, section, roll number) is a separate year-scoped record.

**Guardian.** A first-class entity, not a phone number column on the student. Linked to students through a join table with a relationship type and flags for primary contact and fee responsibility. This is what makes siblings, separated parents and guardians other than parents work correctly.

**Staff.** Teaching and non-teaching. Linked to sections and subjects per academic year.

**Academic calendar.** Working days cannot be derived from the weekday. Tamil Nadu schools lose days to collector-declared rain and cyclone holidays and then recover them through government-directed compensatory working Saturdays, on top of local festival holidays and exam days that may or may not count as instructional.

This matters because **attendance percentage and total working days are printed on the transfer certificate and appear in official registers.** A figure computed from a Monday-to-Friday assumption is simply wrong, and it is wrong on a legal document that follows a child to their next school.

An `academic_calendar_days` entity holds one row per school per date with a `day_type`: `regular_working`, `holiday`, `compensatory_working`, `half_day`, `exam_day`. All working-day totals, attendance percentages and register exports compute from these rows, never from the calendar week.

- The office sets the year's calendar at the start and amends it as holidays are declared.
- Declaring a rain holiday retroactively converts that date, and any attendance already marked for it is preserved but excluded from totals rather than deleted.
- A compensatory Saturday is created by adding a `compensatory_working` row; teachers see the section in their app that day exactly as on any working day.
- Recomputation of affected percentages happens automatically when a day type changes, since a holiday declared on Tuesday afternoon must not leave Tuesday morning's absences counting against a student.

**Class and section.** Class 1 through 12; sections A, B, C within a class. Section assignments are year-scoped.

**Group and medium.** Two attributes of an enrollment that break any model treating a class as a homogeneous cohort:

- **Medium of instruction** (Tamil or English) may vary between sections or between students within a class.
- **Subject group** applies from Class 11, where students in the same class take entirely different subject sets — Computer Science with Maths, Biology with Maths, Commerce with Accountancy, Arts combinations, and school-specific variants.

Both live on the enrollment record, not on the student and not on the section. A student can change group at the start of Class 12, or change medium, and the historical record must stay correct.

**Subject.** Subjects are configured against `(class_id, group_code, medium, academic_year_id)`, not against class and year alone. Modelling subjects as a property of a class works up to Class 10 and then fails completely — two Class 11 students may share a section and a homeroom while taking almost no subjects in common. Mark sheets, report cards and pass/fail rules all resolve through the enrollment's group, so this must be right from the first migration.

For Classes 1–10 the group code is a single default value, so the extra dimension costs nothing there.

**Term and exam.** A term (e.g. Term 1) contains exams (FA1, FA2, SA1). Weighting is configurable per class group.

**Fee head.** A named charge (tuition, transport, exam fee, lab fee) configurable per class and academic year. Never hardcoded — Tamil Nadu private school fees are regulated and heads vary.

**Fee assignment.** The set of fee heads and amounts applicable to a specific student for a year, allowing for concessions, sibling discounts and RTE quota students.

**RTE student.** A distinct category, not a regular student with a 100% discount. Fees work differently, government reimbursement applies, and reporting requirements differ.

### 3.2 Entity relationships

**Outside the tenant boundary (global, no `school_id`, no RLS):**

```
users ──< credentials
      ──< sessions
      ──< user_school_roles ──> schools
```

**Inside the tenant boundary (every table carries `school_id`, RLS enforced):**

```
schools ──< academic_years ──< classes ──< sections
                                 │
students ──< enrollments >───────┘
   │
   └──< student_guardians >── guardians ──> users (by user_id, global)

sections ──< section_teachers >── staff
enrollments ──< attendance_entries
enrollments ──< marks
sections ──< attendance_registers ──< attendance_entries

academic_years ──< terms ──< exams ──< exam_components ──< marks

academic_years ──< fee_heads
students ──< fee_assignments ──< fee_line_items
students ──< payments ──< payment_allocations ──> fee_line_items
```

### 3.2.1 Why identity sits outside the tenant boundary

A guardian is one human being with one mobile number who may have children in more than one school on the platform, and a teacher at School A may be a parent at School B. If the `users` table were tenant-scoped, that person would need two accounts and two logins, and cross-school switching would be impossible without breaking RLS.

Therefore:

- `users`, `credentials`, `sessions` and `user_school_roles` are **global** tables with no `school_id` and no row-level security.
- `guardians` is a **tenant-scoped** record representing that person's relationship with one specific school, holding a nullable `user_id` pointing at the global identity.
- On login the user receives a list of schools they hold a role in. Selecting a school issues a token scoped to that `school_id`; all subsequent tenant queries run under that scope.
- Switching schools requires a new token, never a parameter change on an existing one.

**This is not a hypothetical case in Tamil Nadu.** Trusts commonly run a matriculation higher secondary school alongside a primary or CBSE feeder campus, and staff split timetables across them — the same science teacher takes high school periods at one and middle school at the other. The application must therefore support role switching without logging out: a school selector in the app that exchanges the current token for one scoped to the other school, preserving the session. A teacher forced to log out and back in twice a day will not use the app at either campus.

The same mechanism serves a guardian with children at two schools on the platform, and a teacher who is a parent at the school where they teach.

**Local database isolation is per tenant.** A user holding roles at two schools — a teacher at the trust's matriculation campus who is also a parent at its feeder school — must not have both rosters in one SQLite file. One database file per school (`app_db_<school_id>.sqlite`), each with its own encryption key. Switching schools closes the Drift connection for one and opens the other.

The reason is revocation. When the teacher's staff role is revoked, the wipe must remove that school's roster and nothing else — their parent access at the other school is unaffected and should survive untouched. With a shared file, a `SESSION_REVOKED` for one role either destroys unrelated data or leaves the revoked roster in place. Neither is acceptable, and per-tenant files make the correct behaviour the simple one: delete a file, discard a key.

It also removes any possibility of a local query returning rows from the wrong tenant, which would otherwise be the one place in the system where tenant isolation depends purely on every local query being written correctly.

Because these global tables are unprotected by RLS, every query against them must be explicitly scoped in code and reviewed with the same care as the RLS policies themselves. This is the one place in the system where developer discipline is the only control, so it is deliberately kept as small as possible: four tables, one package, no business logic.

### 3.3 Key schema decisions

- **UUIDs, not sequential integers,** for all public identifiers. Sequential IDs invite enumeration attacks against student records.
- **Soft deletes everywhere.** A deleted student record must remain for audit and historical report cards. `deleted_at` timestamp, never a hard `DELETE`.

  Two rules make this safe rather than decorative:
  - **No `ON DELETE CASCADE` anywhere in the schema.** A cascade is a hard delete by another name, and one accidental parent deletion would silently remove a student's entire attendance and mark history. Foreign keys use `ON DELETE RESTRICT`, so an attempted hard delete fails loudly instead of destroying data quietly.
  - **Exclusion is the default, not the caller's responsibility.** Repository methods filter `deleted_at IS NULL` unless a caller explicitly requests deleted rows. Relying on every query author to remember the predicate guarantees that deleted students eventually reappear in a register or a fee report. Prefer database views over base tables for read paths, so the filter is structural.
- **Money as integer paise,** never floating point. Reconciliation errors from float arithmetic are unrecoverable trust damage.
- **Payment allocation is explicit.** A payment of ₹5,000 against dues of ₹3,000 tuition and ₹2,000 transport creates two allocation rows. Partial payments and refunds then reconcile correctly.
- **Append-only receipt numbers,** sequential per school per financial year, with gaps never permitted. Voided receipts remain in sequence marked void.

  **Gaplessness requires an explicit mechanism.** `BIGSERIAL` and PostgreSQL sequences guarantee uniqueness but not continuity: they are deliberately non-transactional, so a rolled-back transaction consumes a value permanently and leaves a hole. A missing receipt number in a school's books is an audit finding, and "the database did it" is not an answer anyone accepts.

  Receipt and TC numbers are therefore generated from a counter table, incremented inside the same transaction as the document itself, serialized per school with a transaction-scoped advisory lock:

  ```sql
  SELECT pg_advisory_xact_lock(hashtext('receipt_counter_' || current_setting('app.current_school_id')));
  ```

  The lock releases automatically at commit or rollback, so a failed transaction returns the number to circulation rather than burning it. Locking is scoped per school, so one school's fee counter never blocks another's. Because the lock is held only for the increment, contention is negligible even at a busy counter.
- **Attendance and marks link to `enrollment_id`, never to `student_id` plus `section_id`.** Mid-year section transfers are common — a student moves from 6A to 6B in July. If historical records are keyed by current section, records marked while the student was in 6A will vanish from or misalign in the 6B register. An enrollment record carries an effective date range; a transfer closes one enrollment and opens another, and every attendance entry and mark stays attached to the enrollment that was active when it was recorded.

  **The effective range is a `daterange` column with a GiST exclusion constraint**, not a pair of loose date columns:

  ```sql
  ALTER TABLE enrollments ADD COLUMN period daterange NOT NULL;
  ALTER TABLE enrollments ADD CONSTRAINT no_overlapping_enrollment
    EXCLUDE USING gist (school_id WITH =, student_id WITH =, period WITH &&)
    WHERE (deleted_at IS NULL);
  ```

  The performance benefit on historical register reconstruction is real but secondary. The reason to do this is **integrity**: the constraint makes it structurally impossible for a student to hold two overlapping enrollments. Without it, a botched transfer leaves a student in both 6A and 6B for a fortnight, and every downstream calculation — attendance percentage, fee assignment, mark sheets — silently double-counts. That class of bug is very hard to find after the fact and trivial to prevent here.
- **Section registers are reconstructed by date,** querying enrollments whose effective range covers the target date. A section's October register must show who was in that section in October, not who is in it today.
- **Attendance entries, marks and audit logs are declaratively partitioned by academic year** from the first migration. This is nearly free to do at the start and awkward later: closing a year becomes detaching a partition, and queries against the current year never scan history. It is worth doing now purely because retrofitting partitioning onto a live multi-tenant table is a maintenance window nobody wants.

  Two constraints follow from PostgreSQL's partitioning rules and must be designed for, not discovered:
  - **Every unique constraint and primary key on a partitioned table must include the partition key.** Primary keys are therefore composite: `(academic_year_id, id)`. Any code assuming a single-column primary key on these tables will need rewriting, so establish the convention before writing the first repository method.
  - **Foreign keys from partitioned tables must reference indexed columns**, and the referencing side needs its own index or every partition-boundary operation degrades into a sequential scan. Index `enrollment_id` on `attendance_entries` and `marks` explicitly; do not assume the foreign key creates one, because PostgreSQL does not.

**On archival tiering:** at the v1 design target the largest table grows by roughly 4.5 million attendance rows per year. PostgreSQL on the specified hardware handles far more than that with correct indexing and partitioning. Offloading closed years to columnar files in object storage with a separate query engine would add a second storage format, a second query path, and an export pipeline that must itself be verified — real operational surface in exchange for a problem that will not exist within the planning horizon. Partition now because it is cheap; archive only when a measured query regression justifies it.

---

## 4. Functional requirements

### 4.1 Module: Student and staff registry

**4.1.1 Student admission**
- Capture: name (English and Tamil), date of birth, gender, admission number, admission date, class and section, mother tongue, blood group, address, previous school, RTE quota flag, photo.
- **National identity fields:** PEN (Permanent Education Number, 11 digits), APAAR ID (12 digits), EMIS number, and `name_as_per_aadhaar` stored separately from the display name.
- Auto-generate admission number by configurable pattern, with manual override.
- Duplicate detection on name plus date of birth, warning rather than blocking.

**4.1.1.1 PEN and APAAR handling**

APAAR ("One Nation, One Student ID") is mandatory for every enrolled student in Classes 1–12 under UDISE+ from 2026-27, with PEN as a prerequisite. PEN is generated automatically the first time a student's details enter UDISE+; APAAR is generated from PEN plus Aadhaar authentication.

**Scope boundary: this system stores and reconciles these identifiers. It does not generate them.** Generation happens on the UDISE+ SDMS portal through the school's own login, and no third-party API is available for it. Building toward integration would be building against a door that does not exist.

What the system does provide:
- Storage and validation of PEN and APAAR ID per student.
- **A mismatch report.** APAAR generation fails overwhelmingly on name or date-of-birth mismatch between school records and Aadhaar. Holding `name_as_per_aadhaar` separately lets the system flag every student whose school record diverges, which is the single most useful thing this product can do for that workflow — it turns a portal-by-portal error hunt into one list.
- Tracking of APAAR status per student: not started, consent pending, submitted, generated, failed with reason.
- **Consent record, not consent capture.** Current UDISE+ circulars require a physically signed parental consent form for Aadhaar-based APAAR generation. A digital toggle in the parent app does not satisfy that. The system therefore records that a signed form was received, by whom, on what date, with an optional scan attached — and does not pretend to be the consent mechanism.
- Export of PEN and APAAR status for the office to work through on the portal.

Aadhaar numbers themselves are not stored. The school handles Aadhaar on the government portal; holding Aadhaar in a third-party system creates DPDP exposure with no offsetting benefit.

**4.1.2 Guardian linkage**
- Add one or more guardians per student with relationship, mobile, email (optional), occupation.
- Mark one guardian as primary contact and one as fee-responsible; these may differ.
- Detect existing guardian by mobile number and offer to link rather than duplicate — this is how sibling grouping happens.
- Support two guardians with independent logins and independent notification preferences.

**4.1.3 Bulk import**
- Excel/CSV import for students and staff with downloadable template.
- Row-level validation with a downloadable error report identifying the row, column and reason.
- Dry-run preview showing what will be created before committing.
- This tool is the difference between a two-day onboarding and a two-week one. It is not optional.

**Guardian auto-linking must be defended against dirty source data.** Linking siblings by matching mobile numbers is correct in principle and dangerous in practice, because school spreadsheets in India routinely contain:
- A van or auto driver's number listed as primary contact for a dozen unrelated children.
- The school office number pasted down a column where data was missing.
- Placeholder values — all zeros, all nines, `9999999999`, a repeated sample number.
- A single number shared by an extended family living together, where the children genuinely are related but not siblings for fee purposes.

If auto-linking runs unchecked on this, the import silently creates one guardian account with access to thirty unrelated children's records. That is a data breach produced by a convenience feature.

Import heuristics, applied before any merge:
- **A number linking more than four students blocks automatic merging** and is listed for manual verification. Surname and address divergence across the group raises the flag further but does not by itself clear it.
- **Known placeholder patterns are rejected outright** — repeated digits, sequential digits, numbers failing Indian mobile format validation — and the rows import with no guardian contact rather than a false one.
- **The office reviews flagged clusters in a dedicated screen** before any linkage is committed, seeing student names, classes and addresses side by side.
- Auto-linking after go-live, when a new admission matches an existing guardian number, presents a confirmation to the office rather than merging silently.

This screen is worth building well. It is the office's first substantial interaction with the product, and it is where they decide whether it is careful or careless.

**4.1.4 Promotion**
- End-of-year bulk promotion: select source section, target section, review list, exclude detained students, commit.
- Creates new enrollment records for the new academic year; does not modify historical records.
- Handles transfer-out and dropout as terminal enrollment states.

**4.1.5 Staff records**
- Personal details, qualification, designation, date of joining, subjects taught, sections assigned.
- Staff attendance (present/absent/on-duty/leave), separate from student attendance.

### 4.2 Module: Attendance

**4.2.1 Marking (mobile, offline-first)**
- Class teacher opens their section, sees a grid of students with photos, all defaulted to present.
- Tap to toggle a student to absent; long-press for a reason (sick, permitted, unexcused).
- Single confirm action saves the whole register.
- **Writes to local SQLite first and confirms immediately to the user.** Background sync flushes to the server when connectivity returns.
- Visible sync status per register: pending, synced, conflict.
- Half-day and period-wise attendance supported for higher classes, configurable per school.

**4.2.2 Correction**
- Class teacher may correct the current day's register.
- Corrections to prior days require office admin authority.
- Every correction writes an audit entry with before/after values and reason.

**4.2.3 Parent notification**
- Automated absence notification to the primary guardian, dispatched at a school-configured time (default 11:00 AM).
- Notification only after the register is synced and confirmed, never on a provisional local record.

**4.2.4 Reporting**
- Daily section-wise and school-wise present/absent counts.
- Per-student attendance percentage over a date range.
- Consecutive-absence report (default threshold: 3 days) flagged to office and class teacher.
- Monthly attendance register export as PDF matching the school's existing paper format.

**4.2.5 Offline sync rules**
- Local database is the source of truth for reads. Reads never conditionally hit the network.
- Writes queue locally with a monotonic operation ID and flush in order.
- Sync is triggered by connectivity change, app foreground, and a periodic background task.

  **Background workers cannot be relied upon on the devices your teachers actually own.** Xiaomi MIUI, Vivo FunTouch, Oppo ColorOS and Realme UI — dominant in this market — apply aggressive battery management that defers or kills `WorkManager` jobs for hours, silently and without a callback. A design that assumes periodic background sync will lose attendance for a day and give no indication why.

  Therefore:
  - **Foreground and connectivity events are the primary sync triggers**, executed immediately and synchronously, not scheduled. Opening the app flushes the outbox.
  - The periodic worker is a supplement, treated as best-effort. Nothing depends on it running.
  - **The teacher can always see and force it.** A visible pending count with a manual sync action means a teacher who suspects something is wrong has a remedy that does not involve calling you.
  - Onboarding tells teachers to exempt the app from battery optimisation, with a one-tap prompt to the OEM setting. Expect roughly half to skip it, and design so that it does not matter.
- Pending registers older than 24 hours raise an in-app warning to the teacher.

**Conflict resolution is per student entry, not per register.**

Last-write-wins by server receipt time is wrong here and would cause real data loss. Consider: a class teacher marks 6A at 08:45 offline on a weak connection. At 09:00 the office admin records a student as on approved sick leave through the web panel. The teacher's phone finally syncs at 09:15. Under register-level last-write-wins, the teacher's stale offline register silently overwrites the admin's legitimate correction — purely because of network delay, and with no one aware it happened.

The rules are therefore:

1. **Sync granularity is the individual student entry**, keyed by `enrollment_id` and date. A teacher's sync writes only the entries they actually changed, never the whole register.
2. **Ordering uses a per-device revision counter, not wall-clock time.** Android devices drift, users change the clock, and battery-saver modes desync NTP — a 15-minute tolerance is not survivable. Each edit carries `(device_id, local_counter, client_timestamp)` where `local_counter` increments monotonically per device and never resets. The server holds the last counter seen per device per entry and rejects anything at or below it, which makes replays and out-of-order arrivals safe without trusting the clock at all. The client timestamp is retained for display and audit but never decides a conflict.
3. **The server's own revision sequence per entry is authoritative.** Each entry carries a server revision incremented on every accepted write. A client edit declares the server revision it was based on; if the server has moved past it, that edit is a conflict rather than an update.

   **This check is evaluated per entry, never per batch.** A teacher syncing 50 students sends 50 independent edits, each declaring its own base revision. If an admin has since amended student 12, only student 12's edit is rejected; the other 49 apply. Rejecting the whole batch because one entry conflicted would mean a single admin correction silently discards an entire period's attendance — the exact failure this design exists to prevent. The sync endpoint therefore returns a per-entry result set (applied, superseded, invalid), not a single status, and the client reconciles entry by entry.
4. **A server-side change made after the client's base revision wins**, regardless of arrival order.
5. **A change by a higher-authority role wins over a lower one.** Office admin corrections outrank class teacher entries.
6. **When an incoming edit is rejected, the teacher is told.** The app shows which students were superseded, by whom, and what the current value is. Silent rejection is as damaging as silent overwrite.
7. Every superseded version is written to the audit log with both values and the reason for rejection.

**On payload format:** sync uses JSON with gzip compression, not a binary format. A 60-student register is a few kilobytes; gzipped JSON captures nearly all of the available saving because the repeated keys that inflate JSON are exactly what compression eliminates. Adopting MessagePack or Protocol Buffers would add a schema toolchain, a code generation step, and payloads that cannot be read in a log during a production incident — real costs against a marginal gain at this size. Revisit only if measurement on a real 2G connection shows payload size is actually the bottleneck, which is unlikely; connection establishment and round-trip latency dominate on those networks.

### 4.3 Module: Examinations and report cards

**4.3.1 Exam configuration**

A single `max_marks` field per subject is insufficient and will require a schema migration mid-project. Tamil Nadu matriculation schools follow Samacheer Kalvi with Continuous and Comprehensive Evaluation, and the assessment structure differs by stage:

- **Classes 1–8:** school-defined formative and summative assessments. No state-mandated split, so fully configurable.
- **Classes 9–10:** formative assessment split into activity-based FA(a) and unit-test-based FA(b), plus summative terminal examinations, with formative and summative weightings set by state guidance.
- **Classes 11–12:** subjects carry 100 marks composed of separate components. Subjects with practicals are 70 theory + 20 practical + 10 internal assessment; subjects without practicals are 90 theory + 10 internal. Pass mark is 35 overall, with a separate minimum in theory for practical subjects.

**Therefore marks are modelled as components, not as a single score.**

- An `exam_component` record defines: name, type (theory, practical, internal, activity, unit test), maximum marks, minimum pass marks, and weight toward the subject total.
- A subject in a given class and academic year has one or more components. A Class 5 subject may have exactly one; a Class 12 science subject has three.
- Component sets are configurable per class group per academic year, because state guidance changes and the school will not accept a code deploy to reflect it.
- Aggregation rules (weighted sum, best-of-N, drop-lowest) are configuration, not code.
- Both mark-based and grade-based components supported, for co-scholastic areas graded A–E.
- Pass/fail evaluation supports compound rules: minimum in a specific component *and* minimum overall.

Verify the exact current weightings with the school and against Directorate of Government Examinations guidance before configuring, since these are periodically revised.

**4.3.2 Marks entry (mobile and web)**
- Subject teacher selects exam and section, sees a single scrollable list with one numeric field per student.
- Offline-capable with the same local-first pattern as attendance.
- Validation against maximum marks at entry time.
- Absent and exempted are distinct states, not zero.
- Save-as-draft; explicit submit locks the sheet from further teacher edits.

**4.3.3 Report card generation**
- HTML/CSS templates rendered to PDF, so a new school's format is a styling task rather than a code change.
- Configurable per class group: subject list, weighting, grade bands, attendance summary, remarks section, signature blocks, school letterhead.
- Bulk generation runs as a background job; the office is notified when the batch is ready.
- Draft and published states. Parents see only published report cards.
- Generated PDFs stored in object storage and served through short-lived signed URLs.

**4.3.4 Analysis**
- Section and subject-wise mark distribution.
- Student performance trend across terms.
- Subject-wise pass percentage per section, for the principal's dashboard.

### 4.4 Module: Fees

This module carries the highest trust risk in the product. Every requirement here is a correctness requirement.

**4.4.1 Fee structure configuration**
- Fee heads defined per academic year, assignable per class.
- **Every fee head carries a statutory category** — tuition, special, laboratory, library, computer, development, transport, examination, other — in addition to its display name. The Tamil Nadu fee determination committee requires head-wise income filings, and categorising at definition time means the annexure export is a query rather than a manual reclassification exercise every filing cycle. Categories are a fixed enumeration; display names are free text.
- Instalment schedule with due dates per head.
- Concession types: sibling, staff ward, merit, management, RTE. Each concession recorded with an approver and a reason.

**Sibling concession has a dependency the system must track.** The discount is typically applied to the younger child's tuition and is conditional on the elder child's continued enrollment. When the elder child leaves — TC issued, graduated from Class 12, or dropped out — the younger child's concession is no longer automatically valid.

The system must neither continue applying it silently nor cancel it automatically. Both are wrong: silent continuation costs the school money it may not intend to forgo, and automatic cancellation raises a family's fees without anyone deciding to. Instead:

- The concession record stores the **elder child's `student_id`**, not their enrollment.

  This distinction matters more than it looks. Enrollments are year-scoped and close at promotion, so a dependency on `enrollment_id` would treat every elder sibling as having left the school each June, dumping every sibling concession in the school into `review_required` at exactly the moment the office is busiest with promotion and new-year fee demands. The office would learn to click through the warnings without reading them, which destroys the control.

  The dependency resolves at demand-generation time as: does this student have an active enrollment in the current academic year? A promoted student does. A student who took a TC or completed Class 12 does not.
- When the dependency ends, the concession moves to a `review_required` state.
- The next fee demand generation for the younger child raises a blocking warning to the office, showing the concession, why it is in question, and requiring an explicit decision to continue, cancel, or convert it.
- The decision, whoever makes it and whatever it is, is recorded with an approver in the audit log.

The same pattern applies to staff-ward concessions when the staff member leaves.
- Fee structures are versioned; changing a structure mid-year does not retroactively alter issued demands.

**4.4.2 Fee assignment**
- On enrollment, applicable heads are assigned to the student, producing dated line items.
- Individual overrides permitted with reason and approver, recorded in the audit log.

**4.4.3 Payment collection**
- **Offline collection is first-class.** Cash and cheque entry with receipt generation is the primary flow, because this is how most fees are actually collected.
- Online payment via UPI and cards through a gateway registered in the school's name, so settlement is direct to the school.

**4.4.3.1 UPI intent link as the preferred online path**

Gateway MDR of roughly 1.5–2% on a school collecting ₹3 crore annually is ₹4.5–6 lakh. A UPI intent link or dynamic QR carrying a unique transaction reference avoids most of that:

```
upi://pay?pa=school@bank&pn=SchoolName&am=5000.00&tr=<unique_ref>&tn=Fees
```

The `tr` value is generated per demand, stored against the fee line items it covers, and is what makes reconciliation automatic rather than manual.

**One correction worth stating plainly, because it determines whether this works at all:** a bare UPI deep link to a plain VPA gives the school no server-side notification. Money arrives in the bank account and your system never learns of it. Automatic reconciliation requires *either* a bank API or PSP that pushes payment webhooks against your reference, *or* a UPI aggregator offering a low-MDR or zero-MDR UPI-only plan. Confirm what the school's bank actually offers before designing around this. If no notification channel exists, the honest fallback is a link plus a parent-submitted UTR that the office verifies against the bank statement — cheaper than a gateway but not automatic, and it must be presented to the school as such.

Design the payment module so the gateway path and the UPI path are two implementations of one interface. Ship the gateway first because it definitely works; add UPI when the bank capability is confirmed.

**4.4.3.2 Webhook ingestion**

All inbound payment notifications land in a `webhook_events` table before any business logic runs: provider, idempotency key, raw payload, signature verification result, received timestamp, processed timestamp, retry count, error.

- **Idempotency key unique-constrained.** Providers retry aggressively on timeout; a duplicate must be impossible to process twice, enforced by the database rather than by an application check.
- Signature verified before parsing. An unverified payload is logged and discarded.
- Processing is asynchronous, so a slow allocation never causes the provider to time out and retry.
- Unmatched payments — a reference that maps to no known demand — go to an exceptions queue for the office rather than being silently dropped or auto-allocated.
- Raw payloads retained for reconciliation disputes.
- Payment allocation against specific line items, with partial payment support.

**4.4.3.3 Advance and credit balances**

A parent hands over ₹10,000 against a bill of ₹8,600 and tells the clerk to keep the balance for next term. This is routine at every school counter, and a system that only supports exact line-item allocation forces the clerk to either refuse the money or fabricate an allocation — which corrupts the reconciliation the rest of this module depends on.

- A `student_credit_balances` ledger holds unallocated amounts as a first-class balance, credited by overpayment and debited when a future demand is generated.
- The receipt shows the split explicitly: amount allocated to current dues, amount held as advance, and the resulting credit balance. The parent leaves the counter holding a document that matches what they were told.
- On next demand generation, available credit is applied automatically, with the allocation visible and reversible.
- Credit is refundable, subject to the same authority and audit rules as any refund.
- **Credit balances appear in the dues report as a distinct column**, never netted silently into a student's outstanding figure, so the office can see who is in advance and who is in arrears at a glance.
- On a student leaving, an unrefunded credit balance blocks TC issuance with an override, since sending a family away with the school holding their money is the kind of thing that becomes a complaint.
- Immediate printable receipt in two formats: A5 PDF for filing, and 80mm thermal roll for the counter.

**Thermal printing is a specific engineering task, not a CSS afterthought.** Browser `window.print()` against an 80mm ESC/POS roll printer routinely misaligns columns, injects browser headers and footers, and cuts lines mid-row. Requirements:
- Generate raw ESC/POS byte sequences server-side for the receipt, so output is deterministic across printer models.
- Provide a tested print path from the Next.js panel — either a small local print helper or a dedicated print view with a stylesheet built and physically tested against an 80mm continuous roll at the school's actual printer.
- Test on the school's printer before go-live. Receipt printing failing at the fee counter on a due date is the most visible possible failure.
- **Serialize print jobs.** A clerk who sees no immediate output clicks Print again, and two concurrent raw byte streams to an 80mm serial or USB device interleave into garbage — wasting roll and, worse, producing a receipt the parent may accept as valid. The print bridge holds a single-writer lock, queues jobs, debounces repeat clicks on the same receipt, and shows an explicit in-progress state so the clerk knows something is happening. Reprint is a deliberate, separately labelled action that marks the output as a duplicate.
- Cheque lifecycle: received, deposited, cleared, bounced. A bounced cheque reinstates the dues and notifies the office.
- **Cheque return charge.** Banks levy a return charge (typically ₹250–₹500) which schools pass to the parent. On marking a cheque bounced, the system offers to add a configurable "cheque return charge" line item to the student's dues, with the amount defaulting from school configuration and editable per instance. The charge is a normal fee head so it appears in dues, receipts and reconciliation like any other. Adding it is optional per instance — some schools waive it for first occurrences — and the decision, including a waiver, is recorded in the audit log with the approver.

**4.4.4 Refunds and voids**
- Void requires correspondent-level authority and a mandatory reason.
- Voided receipts remain in the sequence; the number is never reused.
- Refund creates a negative allocation, never deletes the original payment.

**4.4.5 Dues management**
- Real-time outstanding dues per student, section, class and school.
- Ageing report: dues by days overdue.
- Automated reminders to fee-responsible guardians, schedulable, with a per-school opt-out and a hard cap on frequency.
- Defaulter list export for the office.

**4.4.6 Reconciliation and audit**
- Daily collection report by mode (cash, cheque, UPI, card) and by collector.

**4.4.6.1 Day-end cash drawer closing**

Cash is where trust in a fee system is actually won or lost. A clerk collects ₹85,000 across twenty students, voids one receipt for a clerical error, and hands the drawer to the correspondent at 16:30. If the physical count and the system total disagree and there is no structured way to see why, the correspondent stops trusting the software — and that is the person who renews your contract.

- **Blind count.** The clerk enters a denomination breakdown — counts of ₹500, ₹200, ₹100, ₹50, ₹20, ₹10, and coins — **before** the system reveals the expected total. Showing the expected figure first turns counting into confirmation bias; every discrepancy would be quietly rounded into agreement.
- The system then displays expected versus counted, the variance, and the day's voided receipts with their reasons, so a shortfall has an immediate audit trail to check against.
- **Two recounts allowed before the variance is locked in.** Miscounting a bundle on the first pass is normal, and forcing a written explanation for what turns out to be a counting error trains clerks to distrust the flow. The clerk may recount and resubmit twice; each attempt is retained in the record, so a genuine discrepancy is still visible rather than counted away. After the third submission the variance stands and the explanation dialog is enforced.
- A variance beyond a configurable tolerance requires a written explanation before closing.
- **A Day-End Closing Voucher** is generated for signature by the clerk and the receiving correspondent, with the denomination breakdown, receipt range covered, voids listed, and both totals.
- **Closing freezes that day's cash batch.** No further cash entry, no voiding, and no back-dating into a closed day. A correction after close is a new dated entry in the current day, never an edit to a settled one.
- Multiple collectors each close their own drawer; the day is closed when all are.
- Unclosed drawers from prior days appear as a blocking item on the office dashboard.

The value here is not fraud prevention so much as blame prevention. When the count is short, this flow tells everyone within two minutes whether it was a void, a miscount or something else — instead of leaving a clerk under suspicion.
- Gateway settlement reconciliation: match gateway payouts against recorded payments and flag discrepancies.
- Every fee transaction is immutable once written. Corrections are new compensating entries.
- Full audit log: who recorded what, when, from which device.

**4.4.7 Regulatory**
- Fee heads and amounts exportable in a format suitable for the state fee determination committee.
- RTE students tracked as a distinct category with reimbursement status.

### 4.5 Module: Communication

**4.5.1 Notices**
- Compose to: whole school, class, section, individual student, or a custom list.
- Attachment support (PDF, image).
- Scheduled send.
- Delivery and read tracking per recipient.
- Tamil and English composition.

**4.5.2 Channels**
- **Push notification via FCM as the primary channel.** Free, unlimited, and the reason parent communication does not destroy the operating budget.
- SMS fallback for guardians without the app, sent through the school's own DLT-registered gateway so the cost sits with the school.
- Per-guardian channel preference and per-category opt-out (fee reminders, attendance, general notices), which is also a DPDP consent requirement.

**4.5.3 Automated notifications**
- Absence alert, fee due reminder, fee overdue reminder, payment receipt confirmation, report card published, exam schedule published.
- Each type independently toggleable per school.

**4.5.3 Emergency broadcast**

Tamil Nadu's monsoon and cyclone season produces same-morning rain holidays announced by district collectors, often before 07:00. Reaching every family within minutes is a genuine operational requirement, not a nice-to-have, and it is the single most visible thing the app will ever do.

- A distinct message class, restricted to correspondent and office admin, requiring an explicit confirmation step.
- **Bypasses quiet hours and all frequency caps.** This is the only message type permitted to do so.
- Dispatched to a priority queue ahead of all routine notifications, with FCM high-priority delivery.
- **Automatic SMS rollover:** any recipient whose push is not acknowledged within a configurable window (default 3 minutes) is sent an SMS, using a pre-approved `EMERGENCY_HOLIDAY` DLT template. This template must be registered during Phase 1 alongside the routine ones, because approval cannot be obtained on the morning of a cyclone.
- Target: all recipients reached within 3 minutes for a 1,500-student school.
- Delivery report visible immediately, so the office knows who has not been reached.
- Load-tested before the monsoon season, not after.

**4.5.4 Message discipline**
- Quiet hours: no automated notification between 21:00 and 07:00.
- Maximum automated messages per guardian per day, configurable, defaulting to three.
- Automated messages never replace a human decision on sensitive matters (detention, disciplinary action).

### 4.6 Module: Certificates and documents

**4.6.1 Transfer certificate**

A TC in Tamil Nadu is a legal document, not a formatted letter. Its fields must be structured columns in the student master with validation, not free-text template variables filled in at generation time — a missing or wrong field on an issued TC is a problem for the family years later, at admission or verification.

Required structured fields:
- Name of student, father's and mother's name, in English and Tamil
- Date of birth in figures and in words
- Nationality, religion, and community category (OC / BC / BCM / MBC / DNC / SC / SCA / ST), with the community certificate number issued by the Revenue Department or e-Sevai, the designation of the issuing authority (Tahsildar, Zonal Deputy Tahsildar), and a flag for whether the certificate has been sighted and verified. These specific details are cross-checked during recognition renewals and inspections, so they belong in the student master rather than being reconstructed from a paper file each time.

  Note that this is sensitive personal data of a minor. It is collected because state education requirements make it necessary for the TC and for inspections, it is visible only to office admin and correspondent roles, every access is audit-logged, and it appears in no export that does not require it. Do not surface it on teacher or parent screens.
- Two personal marks of identification
- Class last studied and class to which promoted
- Whether qualified for promotion
- Date of admission and date of leaving the school
- Reason for leaving
- Number of school days and days present in the current academic year
- Whether fees have been cleared
- Conduct and character
- Whether the student has been subject to any disciplinary action

Additional requirements:
- **Serial numbering is configurable per school**, in one of two modes:
  - **Perpetual** — a single continuous sequence that never resets, matching the physical TC book register most Tamil Nadu schools maintain. This is the common case.
  - **Annual** — reset each academic year with a year prefix, e.g. `TC/2026-27/001`.
  
  The mode is set once at onboarding and locked thereafter; changing it mid-life would break continuity with the paper register the school still keeps. Perpetual mode supports a configurable starting number so the sequence continues from wherever the school's existing book left off, rather than restarting at 1 and creating two documents with the same number years apart.
- Numbers are never reused and gaps are not permitted.
- TC issuance blocked, with an override requiring correspondent authority, when dues are outstanding.
- Issuing a TC closes the student's enrollment with a leaving date; it does not delete the student record.

**EMIS relief is part of the TC workflow, not an afterthought.** Issuing the paper TC is only half the transfer. Until the school relieves the student's profile on the EMIS portal so it moves to the state's common pool, the destination school cannot enrol them — and the family discovers this weeks later, at the new school, and comes back angry at yours.

- An `emis_transfer_status` on the TC record: `pending_relief`, `relieved_to_common_pool`, `acknowledged_by_destination`.
- Issuing a TC creates the record in `pending_relief` and raises it on the office dashboard until cleared.
- A guidance banner walks the clerk through the portal steps, since this is done infrequently enough that nobody remembers the sequence.
- Ageing alert if a TC remains in `pending_relief` beyond a configurable window (default 7 days).
- The system tracks the status; the relief itself is performed on the government portal through the school's own login, as with all EMIS operations.
- Duplicate TC issuance is a distinct action, marked as duplicate on the document, recorded separately.
- Validation at generation: refuse to produce a TC with any mandatory field blank rather than printing an empty line.

**4.6.3 Document verification**

Every generated certificate carries a QR code encoding a verification URL with a document ID and a truncated hash of its contents. Scanning it reaches a public endpoint that confirms the document was issued by this school, on this date, to this student, and has not been altered. The endpoint discloses only enough to confirm authenticity — it is not a student record lookup, and it is rate limited.

This costs roughly a day to build and makes a forged TC or bonafide certificate detectable by any receiving institution with a phone.

**What this is not.** DigiLocker and the National Academic Depository receive documents from *boards and issuing authorities*, not from individual schools' management software, and there is no route for a private school's internal report card to be pushed there. APAAR-linked DigiLocker accounts receive board results and the APAAR card itself through government channels the school does not control. Digitally signed PDF/A-1b is therefore not warranted for v1: it requires a certifying authority signing certificate, key custody, and a signing service, for a document nobody currently validates that way.

Do keep documents structurally clean — consistent metadata, stable document IDs, PDF/A-compatible generation where it is free — so that if a route ever opens, the archive is usable. That is the whole of the future-proofing that is justified here.

**4.6.2 Other documents**
- Bonafide certificate, conduct certificate, study certificate.
- Fee payment history statement for income tax purposes.
- All generated as PDFs from HTML templates, stored, and re-issuable with a record of each issuance.

### 4.7 Module: Timetable (display only in v1)

- Office enters the timetable per section and per teacher.
- Teachers see their day's schedule; parents see their child's.
- Substitution marking for absent teachers.
- **Automated conflict-free generation is explicitly out of scope for v1.** It is a materially different and larger project.

### 4.8 Module: Dashboards

**Correspondent dashboard:** total collection this month against target, outstanding dues with ageing, attendance percentage trend, admission count year on year, section-wise strength.

**Office dashboard:** today's collections by mode, pending cheques, dues follow-up list, unsynced attendance registers, pending certificate requests.

**Teacher dashboard:** today's attendance status for their section, pending marks entry, unread notices.

**Parent dashboard:** child's attendance this month, outstanding dues with due date, latest marks, recent notices.

**Parent app multi-child handling.** A parent with two or more children in the school switches between them with a child selector in the app bar, without re-authenticating. All dashboard content is scoped to the selected child. Fee dues additionally offer a combined family view, since fee bills are often settled per family rather than per student. Notifications state which child they concern.

### 4.9 Module: Class diary and homework

This module exists for one reason: homework is what keeps parents in WhatsApp groups. Attendance and marks are checked occasionally; the daily homework note is checked every evening. Without it, the WhatsApp group survives alongside the app and the school ends up running two systems.

- Class teacher posts one diary entry per section per day: free text plus up to one image attachment.
- Composed on mobile in under a minute — this must be faster than typing into WhatsApp or teachers will not switch.
- Subject teachers may append to their section's entry.
- Single daily push at a school-configured time (default 15:30), batching all entries for a child into one notification rather than one per subject.
- Parents view the last 30 days; older entries archived.
- Not a homework tracker: no submission, no marking, no completion status. Adding those turns a five-day module into a five-week one and is not what displaces WhatsApp.

Scheduled for Phase 4. It is the highest-leverage feature outside the core four modules and should not slip further.

---

## 5. Non-functional requirements

### 5.1 Performance

| Operation | Target |
|---|---|
| App cold start to usable | Under 3 seconds |
| Attendance screen load (60 students) | Under 500 ms from local cache |
| Attendance save (local) | Instant, under 100 ms |
| API response, p95 | Under 300 ms |
| Report card batch, 500 students | Under 10 minutes, background |
| Fee receipt generation | Under 2 seconds |

Load profile is spiky, not steady. Attendance concentrates between 08:30 and 09:15; fee collection concentrates on due dates. Capacity planning targets the spike.

### 5.2 Availability

- Target 99.5% during school hours (07:00–17:00 IST, working days).
- Planned maintenance only on Sundays or holidays.
- The mobile app remains functional for attendance and viewing cached data during a backend outage.
- Fee collection is the one function that hard-fails during an outage, by design.

### 5.3 Scale assumptions

- v1 design target: 15 schools, 1,500 students each, 22,500 students total.
- Single VPS: 4 vCPU, 8 GB RAM, 100 GB disk.
- Growth beyond this triggers vertical scaling first, then extraction of the report generation worker.

### 5.4 Localisation

- Full Tamil and English interface for parent-facing screens.

  **Bundle the Tamil font in the app; do not rely on the device.** Android 8 devices from budget OEMs, which is what a large share of parents carry, frequently render Tamil conjuncts incorrectly — detached diacritics and dotted-circle placeholders where a combining character failed. A parent whose child's name renders as garbage concludes the app is broken, and they are not wrong. Ship Noto Sans Tamil in Flutter assets and set it explicitly for Tamil text rather than inheriting the system font stack. Test on an actual low-end device, not an emulator, since emulator font stacks are not representative.

  The same applies to generated PDFs: embed the font in the document, or Tamil names on report cards and TCs will render differently depending on what opens them.
- English for staff-facing screens, with Tamil student names stored and displayed correctly throughout.
- Indian numbering format (lakh, crore) in financial displays.
- Indian date format (DD-MM-YYYY) everywhere.
- Currency displayed as ₹ with two decimals.

### 5.5 Image handling

Every image in the system — student photos, class diary attachments, scanned consent forms — is captured on a phone that shoots at 12 to 48 megapixels, producing 4 to 12 MB files. Uploading those raw over a classroom 2G connection times out, and a diary photo that fails to send is a teacher who stops posting diary entries.

All images pass through client-side processing before upload:
- Resized to a maximum of 1024×1024, converted to WebP at approximately 80% quality, targeting under 150 KB per file.
- Processing runs in a Dart isolate so the UI never blocks.
- The compressed file is what enters the upload queue, so a queued upload survives app restart without holding a 10 MB original.
- Student photos additionally cropped to a fixed aspect ratio at capture, since they appear on ID cards and report cards.
- Server-side validation rejects anything above a hard ceiling (2 MB), because a compromised or modified client must not be able to fill storage.
- Thumbnails generated server-side for list views, so a parent scrolling thirty diary entries downloads thumbnails rather than full images.

At roughly 150 KB per diary entry, a 1,500-student school across all sections generates a few hundred megabytes a year. Uncompressed, the same usage would run to tens of gigabytes, consume parents' mobile data, and make the diary feel slow enough to abandon.

### 5.6 Device support

- Android 8.0 and above.
- Designed for 720p screens and 2 GB RAM devices.
- APK size under 30 MB.
- Functional on 2G/3G connections.

---

## 6. Security requirements

### 6.1 Tenant isolation

- PostgreSQL row-level security enabled on every table containing school data.
- The application sets the tenant context per request, derived exclusively from the authenticated token and never from a request parameter or body.
- A forgotten `WHERE school_id` clause returns zero rows rather than another school's data.

**Connection pooling is the failure mode that makes RLS dangerous rather than safe.**

Session-level `SET app.current_school_id` is safe only on a dedicated connection held for the life of a request. With `pgxpool`, PgBouncer, or any transaction- or statement-level pooling layer, connections are returned to the pool and handed to the next request — which may belong to a different school. A session variable set by request A and not cleared is still set when request B borrows that connection. This produces silent, intermittent cross-tenant reads that will not appear in testing and may run undetected for months.

Mandatory rules:

1. **Every RLS-dependent query runs inside an explicit transaction**, with `SET LOCAL app.current_school_id` as the first statement. `SET LOCAL` is scoped to the transaction and is discarded at commit or rollback, so nothing leaks to the next borrower of that connection.
2. **No RLS-dependent query may run outside a transaction.** The data access layer exposes no method that reaches the database without one. A single-statement read still opens a transaction.
3. **The tenant context is threaded through request context**, never passed as a function argument that a caller could supply by hand.
4. **Defence in depth: the query builder also injects `school_id` into every tenant-scoped query.** RLS is the backstop, not the only control. Two independent mechanisms must both fail before a leak occurs.
5. **Policies reference a single helper function, not `current_setting` inline.**

   ```sql
   CREATE FUNCTION current_school_id() RETURNS uuid AS $$
     SELECT NULLIF(current_setting('app.current_school_id', true), '')::uuid;
   $$ LANGUAGE sql STABLE PARALLEL SAFE;
   ```

   Policies then read `USING (school_id = current_school_id())`. The benefits are hygiene rather than a dramatic planner fix: one definition to audit across dozens of policies, the `missing_ok` argument so an unset context yields NULL and therefore zero rows rather than an error, and an explicit `STABLE` marking. Note that `current_setting` is already `STABLE` in PostgreSQL, so this is not correcting a volatility problem — verify actual plans with `EXPLAIN ANALYZE` against realistic row counts rather than assuming either version performs well. If a policy does defeat an index scan, that shows up in the plan, and the fix is usually the index rather than the function.
6. **Application database roles do not have `BYPASSRLS`,** and are not the table owner — table owners bypass RLS by default in PostgreSQL. Migrations run as a separate privileged role.
7. **PgBouncer, if introduced, runs in transaction pooling mode only**, and the rules above become load-bearing rather than merely good practice.

**Automated cross-tenant tests in CI**, all of which must run on every commit:
- Authenticate as School A, request a School B record by direct ID, assert not found.
- Run a School A request and a School B request concurrently against a pool of size 1, assert neither sees the other's data. This is the test that catches pooling leaks; a sequential test will not.
- Assert that every tenant-scoped table has an RLS policy enabled, by querying the catalog rather than by maintaining a list. A new table added without a policy must fail the build.

### 6.2 Authentication

- Password hashing with argon2id, using library implementations only. No hand-written cryptographic primitives.
- Staff authenticate with password; parents authenticate with mobile OTP as the primary path.
- OTP: 6 digits, stored hashed, 5-minute expiry, maximum 3 verification attempts, rate limited per mobile number and per IP.
- Access tokens short-lived (15 minutes). Refresh tokens are random opaque values stored hashed, rotated on every use, and individually revocable.
- Refresh token lifetime of 30 days, because forcing daily login on a shared classroom device guarantees abandonment.
- Login rate limiting per account and per IP with exponential backoff.
- All sessions revocable from the admin panel; device list visible to the user.

**Immediate revocation is a hard requirement, not a convenience.** A 30-day refresh lifetime combined with an encrypted local roster means a dismissed teacher or a resigning administrator retains student names, guardian phone numbers and attendance history on a personal phone for up to a month. Staff terminations in schools are not always amicable, and this is the scenario a correspondent will ask about directly.

- Each user record carries a `tokens_valid_after` timestamp. Revoking an account, forcing a logout, or resetting a password sets it to now, invalidating every access and refresh token issued before that moment — one write, no token enumeration required.
- Access token validation checks this timestamp. It is cached in Redis for speed, with the PostgreSQL value authoritative, so a cache failure fails closed rather than open.
- Because access tokens are short-lived, the maximum exposure window after revocation is 15 minutes rather than 30 days.
- **The mobile client treats revocation as a wipe signal, not a logout.** On receiving `401` with a `SESSION_REVOKED` reason, the app immediately purges the SQLCipher key from the Keystore, deletes the local encrypted database and any cached files, and returns to the login screen. It does not wait for the user to act.
- The app performs a lightweight session check on foreground, so a revoked device wipes on next open rather than on next sync attempt.
- Revocation is a single action on the staff record — marking a staff member as left revokes access automatically, so an office admin cannot forget the security step while completing the HR one.
- Every revocation is audit-logged with actor and reason.

### 6.3 Authorisation

- Every record fetch verifies that this requester may see this specific record, not merely that they may call this endpoint. Broken object-level authorisation is the most common real-world breach in systems of this type.
- Permissions evaluated server-side only. Client-side role checks are a UX affordance, never a control.
- UUID identifiers throughout to prevent enumeration.

### 6.4 Data protection

- TLS 1.3 for all traffic, certificates managed automatically.
- Database and Redis bound to the Docker network only, never exposed to a public port.
- Local mobile database encrypted with SQLCipher — a lost teacher's phone otherwise carries a full class roster.

  **The key is the whole control, and a key in the APK is no key at all.** A statically derived or embedded key is extractable from a decompiled APK in minutes, which reduces SQLCipher to obfuscation. Requirements:
  - Key generated with a cryptographically secure random source on first launch, unique per installation. Never derived from a device identifier, a user ID, or anything else predictable.
  - Stored in `flutter_secure_storage` backed by the Android Keystore, so key material is held by hardware-backed storage where the device supports it and never touches application-readable disk.
  - **Do not bind the key to biometric authentication.** Keystore keys created with `setInvalidatedByBiometricEnrollment` or strict user-authentication requirements are destroyed when the user enrols a new fingerprint or changes their screen lock. A teacher adding a fingerprint would silently lose their local database, and the app would present it as a wipe with no explicable cause. Use standard hardware-backed storage without biometric binding; the security benefit of binding does not justify spontaneous data loss during ordinary phone maintenance.
  - Handle key-unavailable errors gracefully regardless: if the key cannot be retrieved for any reason, discard the local database, re-authenticate, and resync from the server rather than crashing. The server is always the recoverable source of truth.
  - **Purged on logout and on uninstall**, together with the encrypted database file. A logged-out device retains no roster.
  - Remote wipe: a device marked revoked from the admin panel clears its local database on next launch, before any sync.
  - Key rotation on re-authentication after a password reset.
- Tokens stored in platform secure storage, never in shared preferences.
- Student photos and generated documents served through short-lived signed URLs after an authorisation check, never as public object URLs.
- No secrets in the mobile binary.

### 6.5 Audit logging

- Append-only log, not modifiable through the application.
- Records: actor, action, entity type and ID, before and after values for changes, timestamp, IP, device identifier.
- Mandatory coverage: student record access and modification, all fee transactions, mark entry and modification, attendance corrections, permission changes, report card publication, bulk exports.
- Retained for seven years alongside financial records.

**Volume control, because seven years of full snapshots will not fit.** Two rules:

- **Log changes, not writes.** A routine attendance sync that sets a student present is not an audit event; a *correction* to an already-recorded attendance is. Marks entry is logged; marks *modification after submission* is logged in detail. Applying this distinction removes the overwhelming majority of would-be entries, because routine daily operation vastly outnumbers corrections. Reads of sensitive records are logged; reads of a teacher's own section roster are not.
- **Store deltas, not snapshots.** The entry holds only the keys that changed with their before and after values, not a copy of the untouched row. A full snapshot of a student record on a phone-number change is roughly fifty times the size of the delta, and JSONB toast compression on the partition does the rest.

Estimate and monitor from the first month rather than trusting either the estimate or the compression. A disk-full event stops PostgreSQL accepting writes, and the 80% alert in section 6.7 is what stands between an audit log and an outage.
- Visible to the correspondent, exportable.

### 6.6 Backup and recovery

- Automated nightly encrypted `pg_dump` pushed to offsite object storage.

  **Throttled, and scheduled into the quiet window.** On 4 vCPU and 8 GB, an unthrottled dump running alongside worker processes produces an I/O and memory spike capable of triggering the OOM killer against PostgreSQL itself — a backup job that takes down the database it is protecting. Run between 02:00 and 03:00 IST when traffic is zero, with `nice -n 19` and `ionice -c 3`, compressing to a temporary file before upload rather than streaming under memory pressure. Alert if the job exceeds its expected window, since a slow dump is the early warning for a disk or database problem.

- Encryption key stored separately from the server.
- Retention: 30 daily, 12 monthly, 7 yearly.
- **Restore verification is an automated scheduled job, not a monthly reminder.** Manual verification tasks get skipped in busy months, and the months you skip are the months you most need them. The script runs weekly: pull the latest dump, spin up a throwaway PostgreSQL container, restore, assert schema version and row counts per major table against expected ranges, run one representative query per module, tear down, and report. A failure pages you. A silent pass is the only acceptable normal state.
- Documented recovery procedure with a target of 4 hours to restore service, walked end to end once during Phase 5 rather than read.

### 6.7 Operational security

- SSH key authentication only; password login disabled.
- Unattended security upgrades enabled.
- Dependency scanning in CI; no deployment with known high-severity vulnerabilities.
- Full server configuration in version control as Docker Compose, so a compromised or failed host is rebuilt from git rather than from memory.
- Error tracking configured to scrub personal data before transmission.

---

## 7. Regulatory and compliance

### 7.1 DPDP Act 2023 and DPDP Rules 2025

The Rules were notified on 13 November 2025 with a full compliance deadline of 13 May 2027. Penalties for children's-data violations reach ₹200 crore and are assessed per contravention.

**Roles.** The school is the Data Fiduciary. The developer is a Data Processor acting on the school's instructions. This must be established in writing before any student data is processed.

**Requirements to implement:**
- Verifiable parental consent captured at enrollment before processing a student's personal data, with the consent record stored and timestamped.
- Standalone, plain-language consent notice in Tamil and English stating exactly what data is collected and why.
- Purpose limitation: data collected for school administration is not used for anything else.
- Data minimisation: do not collect fields the school does not actually use.
- **No profiling or targeted advertising involving minors, ever.** No third-party analytics SDKs in the parent-facing app.
- Guardian rights: access, correction, and erasure requests, with a documented process and a 90-day maximum grievance resolution timeline.
- **Grievance officer contact published in the product.** The school as Data Fiduciary must make its grievance contact available to data principals. A fixed, non-dismissible contact card in the parent app settings and in the admin panel footer showing the designated officer's name, role, email and phone — configured at onboarding, editable only by the correspondent, and never blank. Guardian rights are meaningless if there is no visible way to exercise them, and this is the cheapest compliance item in the entire document.
- Breach notification to affected individuals and the Data Protection Board within 72 hours, with no materiality threshold.
- Retention policy: student records retained per the school's statutory obligations, then erased.
- A written data processing agreement covering permitted processing, breach obligations, sub-processors, and data return or deletion on termination.

**Hosting.** Data hosted in an Indian region. Not strictly mandated — the Rules use a negative-list approach to cross-border transfers rather than localisation — but expected by school management and simpler to explain.

### 7.2 Tamil Nadu specific

- Fee structure records maintained in a form suitable for the state fee determination committee.
- RTE 25% quota students tracked as a distinct category with reimbursement status.
- Tamil language support in all parent-facing output, including printed documents.
- Report card formats follow the school's own matriculation pattern; only Classes 10 and 12 have public board examinations, so Classes 1–9 formats are school-defined and must be configurable.

**EMIS (Educational Management Information System).** Tamil Nadu's EMIS portal holds a centralised student and staff record for schools in the state. Whatever reporting the school currently does into EMIS, it will continue to do after this system is deployed — and if the office has to key the same data twice, they will resent the app regardless of how good it is.

Requirements:
- **EMIS student identifier stored as a first-class field** on the student master, with validation and uniqueness within the school.
- **EMIS staff identifier** on the staff master.
- **Bulk export in the school's EMIS upload layout** for student master data, attendance, and enrollment changes, so office staff upload a file rather than re-typing.
- Export format is **configuration, not compiled code**, because government upload layouts change without notice and a format change must not require a release.

Two cautions. First, the exact reporting burden differs between government, aided and private matriculation schools, and this school's actual EMIS obligations must be confirmed with their office rather than assumed. Second, the upload layouts are not stably published, so this work must be scoped only after obtaining the actual current template files from the school. Do not build to a guessed specification. Treat this as discovery work in Phase 1 and implementation in Phase 5, not a commitment made before the templates are in hand.

### 7.3 UDISE+ and APAAR

APAAR is mandatory for every enrolled student in Classes 1–12 from the 2026-27 UDISE+ cycle, with PEN as a prerequisite and Aadhaar authentication plus signed parental consent required for generation. The annual data window has a hard national freeze date, and the portal blocks new-year entries until the previous year's Student Progression Module is finalised and locked.

Implications for this product:
- The school's UDISE+ work is seasonal and deadline-driven. The mismatch report and status export should be built and tested before the school's data window, not during it.
- Promotion data, marks percentages and attendance totals feed the Student Progression Module. The system already holds all of it; exporting it in a form the office can transcribe or upload turns a multi-day task into an afternoon.
- Aadhaar is not stored in this system under any circumstance.
- Consent for APAAR is recorded, not captured — the signed physical form remains the legal instrument.

### 7.4 Financial

- GST registration and invoicing for the software subscription.
- Payment gateway registered in the school's name, so fee settlement flows directly to the school and gateway charges are not borne by the developer.

---

## 8. Technical architecture

### 8.1 Stack

| Layer | Technology | Rationale |
|---|---|---|
| Mobile | Flutter (Android first) | Single codebase, iOS possible later without rewrite |
| Mobile state | Riverpod | Low boilerplate for a solo developer |
| Mobile local DB | Drift (SQLite) with SQLCipher | Type-safe queries, migrations, encryption |
| Admin panel | Next.js | Standard, fast to build, responsive |
| Backend | Go, modular monolith | Low memory footprint, single binary deploy |
| Database | PostgreSQL 16 | Row-level security, JSON columns, mature |
| Cache | Redis | Session cache, rate limiting, transient data only |
| Job queue | PostgreSQL table with `SELECT ... FOR UPDATE SKIP LOCKED` | Durable across restarts; jobs commit in the same transaction as the data that produced them |
| Background worker | Separate Go process | PDF generation, notification dispatch, nightly jobs |
| Object storage | Cloudflare R2 | Zero egress cost, keeps VPS disk for the database |
| PDF generation | HTML/CSS to PDF | Report card format becomes a styling task |
| Reverse proxy | Caddy | Automatic TLS, simple configuration |
| Push notifications | Firebase Cloud Messaging | Free and unlimited; the reason the budget works |
| Deployment | Docker Compose on a single VPS | Whole server reproducible from git |
| Error tracking | Sentry | Crashes on teachers' devices are otherwise invisible |

### 8.2 Backend structure

A modular monolith with enforced package boundaries:

```
/internal
  /auth          identity, credentials, sessions, OTP
  /tenancy       school context, RLS session management
  /students      student records, guardians, enrollment
  /staff         staff records
  /academic      classes, sections, subjects, terms
  /attendance    registers, entries, sync
  /exams         exams, marks, report cards
  /fees          heads, assignments, payments, reconciliation
  /notify        notices, push, SMS dispatch
  /documents     certificate and PDF generation
  /audit         append-only audit log
```

Cross-package access occurs only through defined interfaces. No package imports another's data access layer. This preserves the option to extract a service later without paying distributed-systems costs now.

**Why not microservices:** a single developer has no team-boundary problem to solve, and distributed transactions across fee, notification and audit boundaries would introduce failure modes disproportionate to the scale. Clean module boundaries deliver the same separation and permit extraction when a specific pressure justifies it.

### 8.3 Job queue durability

Redis with default settings holds the queue in memory. On a single VPS with no redundancy, a container restart, an OOM kill or a host reboot silently discards every queued job — meaning unsent absence alerts, unprocessed payment webhooks, and half-generated report card batches, with nothing to indicate anything was lost. Enabling AOF persistence narrows the window but does not close it, and it introduces a second durable store to back up and reason about.

**Background jobs therefore live in a PostgreSQL table**, claimed by workers with `SELECT ... FOR UPDATE SKIP LOCKED`. The properties that matter:

- **Transactional enqueue.** A payment is recorded and its receipt notification is queued in the same transaction. Either both happen or neither does. With a separate queue this is a distributed commit problem, and the failure mode is a payment with no confirmation to the parent — precisely the kind of gap that destroys trust in a fee system.
- **Survives everything.** The queue is backed up with the database and restored with it.
- **Inspectable.** When the office asks why a parent never received a receipt, the answer is a SQL query, not log archaeology.
- **Retry state is durable:** attempt count, last error, next attempt time, all in the row.
- **Dead letter handling:** jobs exceeding the retry limit move to a failed state visible on an operations screen, never silently dropped.

Throughput at this scale is a few thousand jobs a day. PostgreSQL handles orders of magnitude more; there is no performance argument for Redis here.

**Redis remains** for session cache, rate limiting counters and other genuinely transient data — things whose loss on restart costs nothing worse than a cold cache.

### 8.4 Notification dispatch performance

Sequential FCM sends cannot meet the emergency broadcast target. Reaching 1,500 recipients one request at a time, at even 100ms per round trip, takes two and a half minutes before any retry.

Requirements:
- Use FCM's multicast send, batching up to 500 registration tokens per request. 1,500 recipients becomes three requests.
- Persistent HTTP connections with a connection pool; do not establish a new TLS session per batch.
- Batches dispatched concurrently with a bounded worker pool.
- Per-token failures returned by FCM (unregistered, invalid) are handled individually and stale tokens pruned, without failing the batch.
- SMS rollover computed from the per-token results, not from a global success flag.
- Target: under 10 seconds to dispatch to 1,500 tokens, leaving the remainder of the 3-minute window for SMS rollover.
- **Load-tested against real token volume before monsoon season.** A broadcast path first exercised during an actual cyclone is an untested path.

### 8.5 API design

- REST, versioned from the first commit (`/api/v1/`).
- Responses are append-only within a version: fields are added, never renamed or removed.
- **Minimum-supported-version check on login,** so clients running old builds can be told to update. This endpoint must exist from day one, because retrofitting it cannot reach the clients that need it.
- Cursor pagination on all list endpoints.
- Idempotency keys on all write operations, mandatory for payments and attendance sync.

### 8.6 Offline sync design

- Local database is the read source of truth; reads never conditionally hit the network.
- Writes go to local storage first, then to an outbox queue with operation IDs and timestamps.
- Outbox flushes in order on connectivity restoration, verified by a lightweight reachability check rather than by the operating system's network-interface signal alone.
- Soft deletes locally so deletions propagate before purging.
- Server is authoritative on conflict; the superseded version is preserved in the audit log.
- **Attendance and marks are offline-capable. Fee transactions are not** — money is never written optimistically.

---

## 9. Release plan

### Phase 1 — Foundation (weeks 1–4)
Multi-tenant schema with academic year modelling, enrollment effective dating, global identity tables, RLS policies with transaction-scoped tenant context, auth module, bulk import, student and staff registry with TC-required and EMIS fields, admin panel scaffolding. Cross-tenant isolation tests including the concurrent-pool test passing in CI.

**Start these external processes in week 1, because their lead times exceed the phase:**

- **DLT registration and SMS template approval.** Registration with the telecom operator takes two to four weeks, and every template — with its exact header and variable placeholders — is submitted and approved individually. File all seven in week 1:

  | Template | Purpose |
  |---|---|
  | `OTP_LOGIN` | Parent login verification code |
  | `ATTENDANCE_ABSENT` | Daily automated absence alert |
  | `FEE_DUE_REMINDER` | Upcoming instalment notice |
  | `FEE_RECEIPT_CONFIRM` | Payment confirmation with receipt number and amount |
  | `REPORT_CARD_PUBLISHED` | Term result notification |
  | `EMERGENCY_HOLIDAY` | Collector-declared rain or cyclone holiday |
  | `GENERAL_CIRCULAR` | Generic school announcement |

  Draft each with generous variable slots — adding a variable later means a fresh approval cycle. `OTP_LOGIN` is the one that blocks parent authentication entirely, so file it first.

- **Thermal printer proof of concept, in week 1, not Phase 3.** Obtain the make, model and connection type (USB, Ethernet or Bluetooth) of the school's fee counter printer, and build an endpoint emitting 80mm ESC/POS sequences against that actual device. Bluetooth and USB printers reachable only from the counter PC imply a fundamentally different print path from a network printer the server can address directly — and discovering that in Phase 3 means redesigning the receipt flow at the worst moment.

- **Data processing agreement executed before any real data moves.** Sign the DPA with the correspondent on the day student data is first handed over, including for import testing. A spreadsheet of 900 students on your laptop for a trial import is processing personal data of minors, and the agreement must precede it, not follow it.

- **Payment gateway onboarding** in the school's name, which requires their documentation and takes time.

- **Collect the artefacts that block later phases:** physical report cards for every class group, the current fee structure with statutory head categories, the school's TC format and current TC book serial number, Class 11–12 subject group definitions, and the actual EMIS upload templates.

### Phase 2 — Daily use (weeks 5–9)
Attendance with offline sync and per-entry conflict resolution, notices with FCM push, parent app with child dashboard, absence notification.

**Plus a read-only fee dues view.** Attendance alone is a weak reason for a parent to install and keep an app; visibility of what they owe and what they have paid is the strongest one. This phase therefore includes a dues display fed by a simple import of the school's existing fee position — outstanding amount, due date, and payment history. It is a read-only window, not the fee engine: no collection, no allocation, no receipts. That keeps the full fee module in Phase 3 where it belongs while giving parents a reason to open the app from day one.

**Deploy to the design-partner school at the end of this phase** and let real teachers use it while the rest is built.

### Phase 3 — Fees (weeks 10–14)
Fee configuration, assignment, offline and online collection, receipts, dues and ageing reports, reconciliation, reminders. Ship before a fee due date so it is exercised under real conditions.

### Phase 4 — Academics (weeks 15–20)
Exam configuration with components, marks entry, report card templates and generation, certificates with QR verification, timetable display, **class diary and homework**. **Timed to complete before the school's term-end**, which sets the deadline rather than the developer's estimate.

### Phase 5 — Hardening (weeks 21–24)
Dashboards, PEN/APAAR mismatch reporting and status tracking, EMIS and fee committee exports, emergency broadcast with load test before monsoon season, performance tuning, security review, DPDP consent flows, backup restore drill, documentation, staff training material.

### Post-v1 (in priority order, each gated on a paying school asking)
1. Staff leave and timetable-aware substitution suggestion. Genuinely useful — a teacher calls in sick at 07:30 and the principal is filling periods by hand — but it depends on a complete, maintained timetable matrix, which most schools do not keep current in a system. Build it only once the timetable module is demonstrably in daily use, or the suggestions will be wrong and trusted less than paper.
2. Second and third school onboarding, self-serve setup, support tooling.
3. UPI intent payments, once bank webhook capability is confirmed.
4. Transport, library, payroll.
5. PWA offline caching for the admin panel — read-only views only. Offline receipt drafting is explicitly rejected: money is never written optimistically, and a queued receipt is a receipt the parent may have already walked away with.

---

## 10. Success metrics

**Adoption (the metrics that matter most)**
- Teacher daily active rate above 90% of class teachers by week 4 of deployment.
- Attendance registers submitted before 09:30 on 95% of school days.
- Parent app installation above 60% of families within one term.

**Business outcome for the school**
- Outstanding fee dues reduced by 20% compared to the equivalent period last year.
- Office phone enquiries about dues and marks reduced measurably.
- Report card preparation time reduced from weeks to days.

**Technical**
- Crash-free session rate above 99.5%.
- Attendance sync success rate above 99.9% within 24 hours.
- Zero cross-tenant data incidents. This is a binary metric; any occurrence is a product failure.
- Zero unreconciled fee discrepancies at month end.

**Commercial**
- One reference school completing a full academic cycle.
- Renewal at the end of year one.

---

## 11. Risks

| Risk | Impact | Mitigation |
|---|---|---|
| Teachers reject the app after a sync failure loses attendance | Fatal to adoption | Offline-first architecture; visible sync status; never show success on an unsaved write |
| Report card format mismatch discovered late | Weeks of rework | Obtain physical samples and written signoff for every class group before schema design |
| Fee reconciliation discrepancy | Loss of trust, possible loss of client | Immutable transactions, integer paise, explicit allocation, daily reconciliation report |
| Cross-tenant data leak | Business-ending, DPDP penalties | Transaction-scoped RLS plus query-builder filtering; concurrent-pool isolation test in CI |
| Offline sync overwrites a legitimate admin correction | Silent data loss, teacher and office distrust | Per-entry conflict resolution by creation time and role authority; visible rejection feedback |
| DLT template approval not complete when SMS fallback is needed | Parents without the app receive nothing | Start registration and template drafting in week 1 of Phase 1 |
| EMIS export built to a guessed layout | Wasted work, office rejects the app | Obtain actual template files before scoping; format held in configuration |
| Mid-year section transfer corrupts historical registers | Wrong attendance and marks history | Attendance and marks keyed to enrollment with effective dating |
| Queued notifications or webhooks lost on container restart | Unsent alerts, unprocessed payments, no trace | PostgreSQL-backed job queue; transactional enqueue with the originating write |
| Class 11–12 subject groups modelled as class-level subjects | Report cards wrong for higher secondary; schema migration mid-project | Subjects keyed to class, group and medium from the first migration |
| Sibling concession continues after elder child leaves | Revenue leakage or an unexplained fee increase | Dependency tracked; review state blocks demand generation until decided |
| Thermal printer connection type discovered late | Receipt flow redesign at the fee counter | Printer proof of concept in week 1 against the actual device |
| Receipt or TC number sequence develops gaps | Audit finding; school's books do not reconcile | Counter table with transaction-scoped advisory lock, not a database sequence |
| Dirty import data auto-links unrelated students to one guardian | Data breach via a convenience feature | Shared-contact heuristics; manual verification before any merge |
| SQLCipher key extractable from the APK | Local roster readable on a lost phone | Random per-install key in Android Keystore; purge on logout |
| Nightly backup starves the database of memory | Backup job takes down production | Throttled dump in the 02:00–03:00 window with alerting |
| Dismissed staff retain a cached roster for 30 days | Data breach with a named, motivated actor | `tokens_valid_after` revocation; client wipes local database on `SESSION_REVOKED` |
| Cash drawer variance with no audit trail | Correspondent loses trust in the system | Blind denomination count, day-end voucher, batch freeze |
| Retroactive edit to a closed academic year | Legal instruments invalidated; compliance exposure | Year state machine with RLS-enforced read-only on locked years |
| Uncompressed photo uploads over 2G | Diary abandoned; storage and parent data burned | Client-side resize to WebP under 150 KB before enqueue |
| Working days derived from weekday heuristics | Wrong attendance percentage on TCs and official registers | Calendar entity with explicit day types; all totals computed from it |
| Overpayment with no credit mechanism | Clerks fabricate allocations; reconciliation corrupted | First-class credit balance ledger, shown separately from dues |
| TC issued without EMIS relief | Destination school cannot enrol the child | Transfer status tracked on the TC with ageing alert |
| Audit log fills the disk | PostgreSQL stops accepting writes | Log changes not writes; store deltas; monitor from month one |
| Support load exceeds solo capacity | Burnout, degraded service | Cap client count until support tooling exists; charge for support explicitly |
| Backup fails silently | Catastrophic data loss | Monthly restore verification, not just backup monitoring |
| Scope expansion during build | Missed term deadlines | Written scope in contract; new modules are new commercial conversations |
| Single VPS failure | Total outage | Full configuration in git; documented 4-hour rebuild; offsite backups |

---

## 12. Open questions

1. Exact report card format for each class group — requires physical samples from the school.
2. Current assessment component structure and weightings per class group, confirmed against current DGE guidance rather than assumed.
3. Current fee head structure and instalment schedule for the current academic year.
4. **What EMIS reporting this school actually performs today**, how often, by whom, and the actual upload template files. This determines whether the export work is a half-day or three weeks.
5. The school's exact TC format, to confirm the structured field list is complete.
6. Whether period-wise attendance is required for higher classes or daily attendance suffices.
7. Existing student data format and quality, to size the import work.
8. Number of guardians with smartphones, which determines SMS fallback volume and cost.
9. School's preferred payment gateway and existing merchant relationship.
10. Make and model of the fee counter's receipt printer, to test ESC/POS output against real hardware.
11. Standard cheque return charge the school levies, and whether it is waived on first occurrence.
12. Whether the school wants biometric or RFID attendance in future, which affects the attendance schema.
13. Support hours expectation and escalation path, to be settled in the contract.

---

## Appendix A: Contract items to settle before development

These are not technical requirements but they determine whether the project succeeds commercially.

- Scope definition with an explicit exclusions list.
- Data processing agreement satisfying DPDP obligations.
- Payment gateway registered in the school's name; gateway charges borne by school or parents, never the developer.
- SMS gateway and DLT registration in the school's name.
- Data ownership and export obligations on termination.
- Support scope, hours, response times, and annual maintenance fee.
- Change request process and pricing for out-of-scope work.
- Acceptance criteria and signoff process per phase.
