package agent

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"merit/internal/model"
)

func TestAgentReportsRejectHTTPFailures(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusInternalServerError, http.StatusOK} {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		a, err := New(Config{Manager: ts.URL, APIKey: "key"})
		if err != nil {
			t.Fatal(err)
		}
		for _, reportErr := range []error{a.report(model.AgentStatus{}), a.reportResult(model.TaskResult{})} {
			if (reportErr == nil) != (code == http.StatusOK) {
				t.Fatalf("HTTP %d: unexpected report error %v", code, reportErr)
			}
		}
		ts.Close()
	}
}
