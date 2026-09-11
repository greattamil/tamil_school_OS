package fees

import (
	"fmt"
	"net/url"
)

// UPIIntentLink builds the deep link from PRD 4.4.3.1:
//
//	upi://pay?pa=school@bank&pn=SchoolName&am=5000.00&tr=<unique_ref>&tn=Fees
//
// payeeVPA and payeeName come from school configuration (not modelled yet --
// no school has a real VPA on file in this codebase, see PROGRESS.md); ref is
// the unique reference stored against the fee line items it covers, which is
// what makes reconciliation automatic *if* a notification channel exists for
// it (see the package doc comment on why that's a real "if", not a given).
func UPIIntentLink(payeeVPA, payeeName string, amountPaise int64, ref, note string) string {
	q := url.Values{}
	q.Set("pa", payeeVPA)
	q.Set("pn", payeeName)
	q.Set("am", fmt.Sprintf("%d.%02d", amountPaise/100, amountPaise%100))
	q.Set("tr", ref)
	q.Set("tn", note)
	return "upi://pay?" + q.Encode()
}
