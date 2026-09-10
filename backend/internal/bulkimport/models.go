package bulkimport

// RowError identifies exactly which row/column/reason failed validation, so the
// office gets a downloadable error report they can act on rather than a single
// opaque "import failed" (PRD 4.1.3).
type RowError struct {
	Row    int    `json:"row"` // 1-based, matching what a spreadsheet user sees (header = row 1)
	Column string `json:"column"`
	Reason string `json:"reason"`
}

// row is the parsed-but-unvalidated content of one CSV data row.
type row struct {
	lineNumber int // 1-based, including the header row

	admissionNumber string
	nameEnglish     string
	nameTamil       string
	dateOfBirth     string
	gender          string
	motherTongue    string
	bloodGroup      string
	address         string
	previousSchool  string
	rteQuota        string
	admissionDate   string
	className       string
	sectionName     string
	rollNumber      string
	medium          string
	groupCode       string

	guardianName         string
	guardianMobile       string
	guardianRelationship string
}

// GuardianClusterPreview is one shared-mobile-number cluster found across the
// file, with the decision the auto-linking heuristics reached for it
// (guardianlink.AutoLink / ManualReview / Rejected). The office reviews
// ManualReview clusters in a dedicated screen before any linkage is committed
// (PRD 4.1.3).
type GuardianClusterPreview struct {
	Mobile       string   `json:"mobile"`
	Rows         []int    `json:"rows"`
	StudentNames []string `json:"student_names"`
	Decision     string   `json:"decision"`
}

type PreviewResult struct {
	TotalRows        int                      `json:"total_rows"`
	ValidRows        int                      `json:"valid_rows"`
	Errors           []RowError               `json:"errors"`
	GuardianClusters []GuardianClusterPreview `json:"guardian_clusters"`
}

type CommitResult struct {
	StudentsCreated int        `json:"students_created"`
	GuardiansLinked int        `json:"guardians_linked"`
	Errors          []RowError `json:"errors"`
}
