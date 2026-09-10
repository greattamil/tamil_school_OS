package bulkimport

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"school-erp/backend/internal/academic"
	"school-erp/backend/internal/bulkimport/guardianlink"
	"school-erp/backend/internal/enrollments"
	"school-erp/backend/internal/guardians"
	"school-erp/backend/internal/students"
)

type Service struct {
	students    *students.Repository
	guardians   *guardians.Repository
	enrollments *enrollments.Repository
	academic    *academic.Repository
}

func NewService(s *students.Repository, g *guardians.Repository, e *enrollments.Repository, a *academic.Repository) *Service {
	return &Service{students: s, guardians: g, enrollments: e, academic: a}
}

// Preview parses and validates a CSV without writing anything, so the office sees
// what will be created -- and which guardian-mobile clusters need their sign-off --
// before committing (PRD 4.1.3: "Dry-run preview showing what will be created
// before committing").
func (svc *Service) Preview(ctx context.Context, csv io.Reader) (PreviewResult, error) {
	rows, err := parseCSV(csv)
	if err != nil {
		return PreviewResult{}, err
	}

	result := PreviewResult{Errors: []RowError{}}
	result.TotalRows = len(rows)

	validRowIdx := make([]int, 0, len(rows))
	for i, r := range rows {
		if errs := validateRow(r); len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
			continue
		}
		validRowIdx = append(validRowIdx, i)
	}
	result.ValidRows = len(validRowIdx)

	result.GuardianClusters = buildGuardianClusters(rows, validRowIdx)

	return result, nil
}

// GuardianDecision is the office's explicit call on one ManualReview cluster
// (PRD 4.1.3: "The office reviews flagged clusters ... before any linkage is
// committed"). Action is "link" (create one guardian, link every row in the
// cluster) or "skip" (import those rows with no guardian contact).
type GuardianDecision struct {
	Mobile string `json:"mobile"`
	Action string `json:"action"`
}

// Commit re-validates and then writes: one student per valid row, its enrollment,
// and guardian linkage per the auto-linking heuristics plus any manual decisions
// supplied for clusters that needed review. Each row is processed independently --
// a bad row is reported in Errors and skipped, it does not abort rows already
// written (matching the per-entry-not-per-batch philosophy used elsewhere in this
// system, e.g. attendance sync).
func (svc *Service) Commit(ctx context.Context, csvReader io.Reader, academicYearID uuid.UUID, decisions []GuardianDecision) (CommitResult, error) {
	rows, err := parseCSV(csvReader)
	if err != nil {
		return CommitResult{}, err
	}

	decisionByMobile := make(map[string]string, len(decisions))
	for _, d := range decisions {
		decisionByMobile[d.Mobile] = d.Action
	}

	result := CommitResult{Errors: []RowError{}}

	// guardianIDByMobile caches guardians created earlier in this same commit, so
	// a cluster of five siblings creates exactly one guardian row, not five.
	guardianIDByMobile := make(map[string]uuid.UUID)

	for _, r := range rows {
		if errs := validateRow(r); len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
			continue
		}

		section, err := svc.academic.FindSectionByName(ctx, academicYearID, r.className, r.sectionName)
		if err != nil {
			result.Errors = append(result.Errors, RowError{r.lineNumber, "section_name",
				"no section '" + r.sectionName + "' in class '" + r.className + "' for this academic year"})
			continue
		}

		// Both dates already passed time.Parse in validateRow, so the error here
		// cannot occur; ignoring it avoids a redundant, unreachable branch.
		dob, _ := time.Parse(dateLayout, r.dateOfBirth)
		admissionDate, _ := time.Parse(dateLayout, r.admissionDate)

		var nameTamil, motherTongue, bloodGroup, address, previousSchool *string
		if r.nameTamil != "" {
			nameTamil = &r.nameTamil
		}
		if r.motherTongue != "" {
			motherTongue = &r.motherTongue
		}
		if r.bloodGroup != "" {
			bloodGroup = &r.bloodGroup
		}
		if r.address != "" {
			address = &r.address
		}
		if r.previousSchool != "" {
			previousSchool = &r.previousSchool
		}

		student, err := svc.students.Create(ctx, students.CreateStudentInput{
			AdmissionNumber: r.admissionNumber,
			NameEnglish:     r.nameEnglish,
			NameTamil:       nameTamil,
			DateOfBirth:     dob,
			Gender:          r.gender,
			MotherTongue:    motherTongue,
			BloodGroup:      bloodGroup,
			Address:         address,
			PreviousSchool:  previousSchool,
			RTEQuota:        parseBooleanish(r.rteQuota),
			AdmissionDate:   admissionDate,
		})
		if err != nil {
			reason := "could not create student"
			if errors.Is(err, students.ErrDuplicateAdmissionNumber) {
				reason = "admission number already exists"
			}
			result.Errors = append(result.Errors, RowError{r.lineNumber, "admission_number", reason})
			continue
		}
		result.StudentsCreated++

		var rollNumber *string
		if r.rollNumber != "" {
			rollNumber = &r.rollNumber
		}
		if _, err := svc.enrollments.Create(ctx, enrollments.CreateInput{
			StudentID:      student.ID,
			AcademicYearID: academicYearID,
			ClassID:        section.ClassID,
			SectionID:      section.ID,
			RollNumber:     rollNumber,
			Medium:         r.medium,
			GroupCode:      r.groupCode,
			StartDate:      admissionDate,
		}); err != nil {
			result.Errors = append(result.Errors, RowError{r.lineNumber, "class_name/section_name", "could not create enrollment: " + err.Error()})
			// The student record still exists even though enrollment failed;
			// this is surfaced to the office rather than silently rolled back,
			// since a partial student-without-enrollment row is exactly the
			// kind of thing the error report exists to catch.
		}

		if r.guardianMobile == "" {
			continue
		}

		decision := guardianlink.DecideCluster(r.guardianMobile, clusterSizeForMobile(rows, r.guardianMobile))
		if decision == guardianlink.ManualReview {
			action, decided := decisionByMobile[r.guardianMobile]
			if !decided || action != "link" {
				continue // skip until/unless the office decides to link
			}
		}
		if decision == guardianlink.Rejected {
			continue // row imports with no guardian contact, per PRD 4.1.3
		}

		guardianID, ok := guardianIDByMobile[r.guardianMobile]
		if !ok {
			existing, err := svc.guardians.FindByMobile(ctx, r.guardianMobile)
			if err != nil {
				result.Errors = append(result.Errors, RowError{r.lineNumber, "guardian_mobile", "could not look up guardian"})
				continue
			}
			if len(existing) > 0 {
				guardianID = existing[0].ID
			} else {
				g, err := svc.guardians.Create(ctx, r.guardianName, r.guardianMobile, nil, nil)
				if err != nil {
					result.Errors = append(result.Errors, RowError{r.lineNumber, "guardian_mobile", "could not create guardian"})
					continue
				}
				guardianID = g.ID
			}
			guardianIDByMobile[r.guardianMobile] = guardianID
		}

		relationship := guardians.RelationGuardian
		switch r.guardianRelationship {
		case "father":
			relationship = guardians.RelationFather
		case "mother":
			relationship = guardians.RelationMother
		case "other":
			relationship = guardians.RelationOther
		}

		if err := svc.guardians.LinkToStudent(ctx, guardians.StudentGuardianLink{
			StudentID:        student.ID,
			GuardianID:       guardianID,
			Relationship:     relationship,
			IsPrimaryContact: true,
			IsFeeResponsible: true,
		}); err != nil {
			result.Errors = append(result.Errors, RowError{r.lineNumber, "guardian_mobile", "could not link guardian to student"})
			continue
		}
		result.GuardiansLinked++
	}

	return result, nil
}

func buildGuardianClusters(rows []row, validRowIdx []int) []GuardianClusterPreview {
	mobiles := make([]string, len(rows))
	for _, i := range validRowIdx {
		mobiles[i] = rows[i].guardianMobile
	}
	clusters := guardianlink.ClusterRows(mobiles)

	result := make([]GuardianClusterPreview, 0, len(clusters))
	for mobile, rowIdxs := range clusters {
		names := make([]string, 0, len(rowIdxs))
		for _, i := range rowIdxs {
			names = append(names, rows[i].nameEnglish)
		}
		result = append(result, GuardianClusterPreview{
			Mobile:       mobile,
			Rows:         toLineNumbers(rows, rowIdxs),
			StudentNames: names,
			Decision:     string(guardianlink.DecideCluster(mobile, len(rowIdxs))),
		})
	}
	return result
}

func toLineNumbers(rows []row, idxs []int) []int {
	out := make([]int, len(idxs))
	for i, idx := range idxs {
		out[i] = rows[idx].lineNumber
	}
	return out
}

func clusterSizeForMobile(rows []row, mobile string) int {
	n := 0
	for _, r := range rows {
		if r.guardianMobile == mobile {
			n++
		}
	}
	return n
}
