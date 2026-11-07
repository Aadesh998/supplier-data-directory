package notify

import (
	"strings"
	"testing"
)

func TestTemplatesRender(t *testing.T) {
	data := map[string]any{"CompanyName": "Example Co", "SubmitterName": "A", "SiteName": "BMD", "SiteURL": "https://x.example",
		"SubmissionID": "id", "NoticeVersion": "v1", "SubmittedAt": "now", "PublicListing": true, "CommercialAPI": false,
		"Status": "needs_changes", "Reason": "missing city", "RequestTypeLabel": "correction", "RequesterName": "B",
		"ReceivedAt": "now", "RequestID": "r", "ContactName": "C", "SenderName": "D", "HowFound": "at a trade fair"}
	for _, name := range []string{"submission_received", "submission_approved", "submission_rejected", "privacy_request_received",
		"admin_new_submission", "supplier_invitation"} {
		subject, body, err := Render(Message{Template: name, Data: data})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if subject == "" || body == "" || strings.Contains(body, "<no value>") {
			t.Errorf("%s rendered badly: %q / %q", name, subject, body)
		}
	}
	_, body, _ := Render(Message{Template: "submission_received", Data: data})
	for _, banned := range []string{"guarantee", "leads guaranteed", "top ranking"} {
		if strings.Contains(strings.ToLower(body), banned) {
			t.Errorf("confirmation email must not promise %q", banned)
		}
	}
}
