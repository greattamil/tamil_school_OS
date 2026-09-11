package fees

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"school-erp/backend/internal/db"
	"school-erp/backend/internal/notify"
	"school-erp/backend/internal/tenancy"
)

func (h *Handlers) registerPhase3(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/fees/heads", h.createFeeHead)
	mux.HandleFunc("GET /api/v1/fees/heads", h.listFeeHeads)
	mux.HandleFunc("POST /api/v1/fees/structures", h.createFeeStructure)
	mux.HandleFunc("GET /api/v1/fees/structures", h.listFeeStructures)
	mux.HandleFunc("GET /api/v1/fees/structures/{id}/instalments", h.listInstalments)

	mux.HandleFunc("POST /api/v1/fees/concessions", h.createConcession)
	mux.HandleFunc("GET /api/v1/students/{id}/concessions", h.listConcessions)
	mux.HandleFunc("GET /api/v1/fees/concessions/review-required", h.reviewRequiredConcessions)
	mux.HandleFunc("POST /api/v1/fees/concessions/{id}/resolve", h.resolveConcession)

	mux.HandleFunc("POST /api/v1/students/{id}/fee-assignment", h.generateAssignment)
	mux.HandleFunc("GET /api/v1/students/{id}/fee-line-items", h.lineItems)
	mux.HandleFunc("GET /api/v1/students/{id}/payments", h.paymentsForStudent)

	mux.HandleFunc("POST /api/v1/fees/payments", h.collectPayment)
	mux.HandleFunc("POST /api/v1/fees/payments/{id}/void", h.voidPayment)
	mux.HandleFunc("POST /api/v1/fees/payments/{id}/cheque-status", h.chequeStatus)
	mux.HandleFunc("POST /api/v1/fees/refunds", h.refund)

	mux.HandleFunc("GET /api/v1/fees/dues-ageing", h.duesAgeing)

	mux.HandleFunc("POST /api/v1/fees/cash-drawer/count", h.submitCount)
	mux.HandleFunc("POST /api/v1/fees/cash-drawer/{id}/close", h.closeDrawer)
	mux.HandleFunc("GET /api/v1/fees/cash-drawer/open", h.openDrawers)

	mux.HandleFunc("GET /api/v1/fees/annexure", h.annexureExport)
	mux.HandleFunc("POST /api/v1/fees/assignments/{id}/rte", h.setRTEStatus)
	mux.HandleFunc("GET /api/v1/fees/rte-students", h.listRTEStudents)
}

func requireRole(w http.ResponseWriter, r *http.Request, allowed ...string) bool {
	role, _ := tenancy.Role(r.Context())
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	writeError(w, http.StatusForbidden, "forbidden", "role may not perform this action")
	return false
}

// --- Fee heads & structures (PRD 2.2: "Configure fee structure" -- correspondent only) ---

type createFeeHeadRequest struct {
	AcademicYearID    uuid.UUID         `json:"academic_year_id"`
	Name              string            `json:"name"`
	StatutoryCategory StatutoryCategory `json:"statutory_category"`
}

func (h *Handlers) createFeeHead(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent") {
		return
	}
	var req createFeeHeadRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.AcademicYearID == uuid.Nil || req.Name == "" || req.StatutoryCategory == "" {
		writeError(w, http.StatusBadRequest, "invalid_body", "academic_year_id, name and statutory_category are required")
		return
	}
	head, err := h.repo.CreateFeeHead(r.Context(), req.AcademicYearID, req.Name, req.StatutoryCategory)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, head)
}

func (h *Handlers) listFeeHeads(w http.ResponseWriter, r *http.Request) {
	yearID, err := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "academic_year_id query param is required")
		return
	}
	heads, err := h.repo.ListFeeHeads(r.Context(), yearID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": heads})
}

type createFeeStructureRequest struct {
	AcademicYearID uuid.UUID         `json:"academic_year_id"`
	ClassID        uuid.UUID         `json:"class_id"`
	Instalments    []InstalmentInput `json:"instalments"`
}

func (h *Handlers) createFeeStructure(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent") {
		return
	}
	var req createFeeStructureRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.AcademicYearID == uuid.Nil || req.ClassID == uuid.Nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "academic_year_id and class_id are required")
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	structure, err := h.repo.CreateFeeStructure(r.Context(), req.AcademicYearID, req.ClassID, actorID, req.Instalments)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, structure)
}

func (h *Handlers) listFeeStructures(w http.ResponseWriter, r *http.Request) {
	yearID, err1 := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	classID, err2 := uuid.Parse(r.URL.Query().Get("class_id"))
	if err1 != nil || err2 != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", "academic_year_id and class_id are required")
		return
	}
	items, err := h.repo.ListFeeStructures(r.Context(), yearID, classID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) listInstalments(w http.ResponseWriter, r *http.Request) {
	structureID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid structure id")
		return
	}
	items, err := h.repo.ListInstalments(r.Context(), structureID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// --- Concessions ---

type createConcessionRequest struct {
	StudentID        uuid.UUID      `json:"student_id"`
	ConcessionType   ConcessionType `json:"concession_type"`
	FeeHeadID        *uuid.UUID     `json:"fee_head_id"`
	Percentage       *float64       `json:"percentage"`
	FlatAmountPaise  *int64         `json:"flat_amount_paise"`
	ElderStudentID   *uuid.UUID     `json:"elder_student_id"`
	DependentStaffID *uuid.UUID     `json:"dependent_staff_id"`
	Reason           string         `json:"reason"`
}

func (h *Handlers) createConcession(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	var req createConcessionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	c, err := h.repo.CreateConcession(r.Context(), CreateConcessionInput{
		StudentID: req.StudentID, ConcessionType: req.ConcessionType, FeeHeadID: req.FeeHeadID,
		Percentage: req.Percentage, FlatAmountPaise: req.FlatAmountPaise,
		ElderStudentID: req.ElderStudentID, DependentStaffID: req.DependentStaffID, Reason: req.Reason,
	}, actorID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (h *Handlers) listConcessions(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	items, err := h.repo.ListConcessionsForStudent(r.Context(), studentID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) reviewRequiredConcessions(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	items, err := h.repo.ListReviewRequiredConcessions(r.Context())
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type resolveConcessionRequest struct {
	Decision ConcessionStatus `json:"decision"` // active (continue), cancelled, converted
	Note     string           `json:"note"`
}

func (h *Handlers) resolveConcession(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	concessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid concession id")
		return
	}
	var req resolveConcessionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.ResolveReviewRequiredConcession(r.Context(), concessionID, req.Decision, req.Note, actorID); err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- Fee assignment ---

type generateAssignmentRequest struct {
	AcademicYearID uuid.UUID `json:"academic_year_id"`
}

func (h *Handlers) generateAssignment(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	var req generateAssignmentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	assignment, lineItems, err := h.repo.GenerateAssignment(r.Context(), studentID, req.AcademicYearID, actorID)
	if err != nil {
		var reviewErr *ConcessionReviewError
		if errors.As(err, &reviewErr) {
			writeJSON(w, http.StatusConflict, map[string]any{
				"error": "concession_review_required", "message": reviewErr.Error(),
				"concessions": reviewErr.Concessions,
			})
			return
		}
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"assignment": assignment, "line_items": lineItems})
}

func (h *Handlers) lineItems(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	var yearFilter *uuid.UUID
	if v := r.URL.Query().Get("academic_year_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "invalid academic_year_id")
			return
		}
		yearFilter = &id
	}
	items, err := h.repo.LineItemsForStudent(r.Context(), studentID, yearFilter)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	credit, err := h.repo.CreditBalance(r.Context(), studentID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "credit_balance_paise": credit})
}

func (h *Handlers) paymentsForStudent(w http.ResponseWriter, r *http.Request) {
	studentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid student id")
		return
	}
	items, err := h.repo.PaymentsForStudent(r.Context(), studentID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// --- Payments ---

type collectPaymentRequest struct {
	StudentID          uuid.UUID         `json:"student_id"`
	Mode               PaymentMode       `json:"mode"`
	AmountPaise        int64             `json:"amount_paise"`
	Allocations        []AllocationInput `json:"allocations"`
	AdvanceAmountPaise int64             `json:"advance_amount_paise"`
	ChequeNumber       string            `json:"cheque_number"`
	ChequeBank         string            `json:"cheque_bank"`
	DeviceID           string            `json:"device_id"`
}

func (h *Handlers) collectPayment(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	var req collectPaymentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	payment, err := h.repo.CollectPayment(r.Context(), CollectPaymentInput{
		StudentID: req.StudentID, Mode: req.Mode, AmountPaise: req.AmountPaise,
		Allocations: req.Allocations, AdvanceAmountPaise: req.AdvanceAmountPaise,
		ChequeNumber: req.ChequeNumber, ChequeBank: req.ChequeBank, DeviceID: req.DeviceID,
	}, actorID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}

	// PRD 4.5.3: "payment receipt confirmation" as an automated notification
	// type. Fired synchronously right after the payment commits (there is a
	// real actor here, unlike the reminder scan), targeting every guardian of
	// the student (TargetStudent's normal resolution, not just the
	// fee-responsible one -- a receipt is something every guardian
	// reasonably wants to see, unlike a due/overdue reminder). Best-effort:
	// a failure here must never make an already-recorded, already-receipted
	// payment look like it failed to the clerk at the counter.
	if h.notifyRepo != nil {
		body := fmt.Sprintf("Receipt #%d for %s received.", payment.ReceiptNumber, formatPaiseForNotify(payment.AmountPaise))
		if _, _, err := h.notifyRepo.Compose(r.Context(), notify.ComposeInput{
			Kind:       notify.KindPaymentReceipt,
			Title:      "Payment received",
			BodyEN:     body,
			TargetType: notify.TargetStudent,
			StudentID:  &req.StudentID,
		}, actorID); err != nil {
			log.Printf("fees: payment receipt notification failed for payment %s: %v", payment.ID, err)
		}
	}

	writeJSON(w, http.StatusCreated, payment)
}

type voidPaymentRequest struct {
	Reason string `json:"reason"`
}

func (h *Handlers) voidPayment(w http.ResponseWriter, r *http.Request) {
	// PRD 2.2 role matrix: "Void/refund a payment -- Correspondent: Yes, everyone else: No."
	if !requireRole(w, r, "correspondent") {
		return
	}
	paymentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid payment id")
		return
	}
	var req voidPaymentRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.VoidPayment(r.Context(), paymentID, req.Reason, actorID); err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type chequeStatusRequest struct {
	Status          ChequeStatus `json:"status"`
	AddReturnCharge bool         `json:"add_return_charge"`
	WaiverNote      string       `json:"waiver_note"`
}

func (h *Handlers) chequeStatus(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	paymentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid payment id")
		return
	}
	var req chequeStatusRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.MarkChequeStatus(r.Context(), paymentID, req.Status, req.AddReturnCharge, req.WaiverNote, actorID); err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type refundRequest struct {
	PaymentID   *uuid.UUID `json:"payment_id"`
	StudentID   uuid.UUID  `json:"student_id"`
	AmountPaise int64      `json:"amount_paise"`
	Reason      string     `json:"reason"`
}

func (h *Handlers) refund(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent") {
		return
	}
	var req refundRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	refundID, err := h.repo.RefundPayment(r.Context(), req.PaymentID, req.StudentID, req.AmountPaise, req.Reason, actorID, actorID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"refund_id": refundID})
}

func (h *Handlers) duesAgeing(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	items, err := h.repo.Dues(r.Context())
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// --- Cash drawer ---

type submitCountRequest struct {
	BusinessDate string       `json:"business_date"`
	Denomination Denomination `json:"denomination"`
	Explanation  string       `json:"explanation"`
}

func (h *Handlers) submitCount(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	var req submitCountRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	date, err := time.Parse("2006-01-02", req.BusinessDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_date", "business_date must be YYYY-MM-DD")
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	closing, err := h.repo.SubmitCount(r.Context(), actorID, date, req.Denomination, req.Explanation, actorID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, closing)
}

func (h *Handlers) closeDrawer(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	closingID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid closing id")
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	closing, err := h.repo.CloseDrawer(r.Context(), closingID, actorID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, closing)
}

func (h *Handlers) openDrawers(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	items, err := h.repo.OpenDrawers(r.Context())
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handlers) annexureExport(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	yearID, err := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "academic_year_id query param is required")
		return
	}
	items, err := h.repo.AnnexureExport(r.Context(), yearID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type setRTEStatusRequest struct {
	IsRTE                  bool   `json:"is_rte"`
	RTEReimbursementStatus string `json:"rte_reimbursement_status"`
}

func (h *Handlers) setRTEStatus(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	assignmentID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_id", "invalid assignment id")
		return
	}
	var req setRTEStatusRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}
	actorID, _ := tenancy.UserID(r.Context())
	if err := h.repo.SetRTEStatus(r.Context(), assignmentID, req.IsRTE, req.RTEReimbursementStatus, actorID); err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handlers) listRTEStudents(w http.ResponseWriter, r *http.Request) {
	if !requireRole(w, r, "correspondent", "office_admin") {
		return
	}
	yearID, err := uuid.Parse(r.URL.Query().Get("academic_year_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_academic_year_id", "academic_year_id query param is required")
		return
	}
	items, err := h.repo.ListRTEAssignments(r.Context(), yearID)
	if err != nil {
		writeFeesRepoError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", "invalid request body")
		return false
	}
	return true
}

func writeFeesRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrStructureNotFound), errors.Is(err, ErrPaymentNotFound), errors.Is(err, ErrConcessionNotFound):
		writeError(w, http.StatusNotFound, "not_found", err.Error())
	case errors.Is(err, ErrDuplicateHeadName), errors.Is(err, ErrAssignmentExists):
		writeError(w, http.StatusConflict, "duplicate", err.Error())
	case errors.Is(err, ErrNoActiveStructure):
		writeError(w, http.StatusConflict, "no_active_structure", err.Error())
	case errors.Is(err, ErrDrawerClosed), errors.Is(err, ErrAlreadyVoid), errors.Is(err, ErrDrawerAlreadyClosed),
		errors.Is(err, ErrTooManyRecounts), errors.Is(err, ErrVarianceExplanationRequired), errors.Is(err, ErrDrawerNotReadyToClose),
		errors.Is(err, ErrNotACheque), errors.Is(err, ErrInvalidChequeTransition), errors.Is(err, ErrInsufficientLineItemRoom),
		errors.Is(err, ErrInsufficientCredit):
		writeError(w, http.StatusConflict, "conflict", err.Error())
	case errors.Is(err, db.ErrNoTenant):
		writeError(w, http.StatusForbidden, "no_tenant", "request is not scoped to a school")
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	}
}
