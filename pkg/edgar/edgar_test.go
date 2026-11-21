package edgar

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestLookupCIK(t *testing.T) {
	client := NewClient("")

	// Test 1: Cached ticker
	cik, err := client.LookupCIK("AAPL")
	if err != nil {
		t.Fatalf("expected AAPL lookup to succeed: %v", err)
	}
	if cik != 320193 {
		t.Errorf("expected CIK 320193, got %d", cik)
	}

	cikMSFT, err := client.LookupCIK("msft")
	if err != nil {
		t.Fatalf("expected MSFT lookup to succeed: %v", err)
	}
	if cikMSFT != 789019 {
		t.Errorf("expected CIK 789019, got %d", cikMSFT)
	}
}

func TestFindAndDownloadFiling(t *testing.T) {
	// Mock SEC Submissions endpoint
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/submissions/CIK0000320193.json" {
			sub := secSubmissions{
				CIK:        "0000320193",
				EntityName: "Apple Inc.",
			}
			sub.FilingsRecent.Form = []string{"10-K", "10-Q"}
			sub.FilingsRecent.AccessionNumber = []string{"0000320193-24-000106", "0000320193-24-000050"}
			sub.FilingsRecent.FilingDate = []string{"2024-11-01", "2024-05-03"}
			sub.FilingsRecent.ReportDate = []string{"2024-09-28", "2024-03-30"}
			sub.FilingsRecent.PrimaryDocument = []string{"aapl-20240928.htm", "aapl-20240330.htm"}
			sub.FilingsRecent.Size = []int64{1234567, 987654}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sub)
			return
		}

		if r.URL.Path == "/test-download.txt" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("MOCK SEC FILING DATA STREAM"))
			return
		}

		http.NotFound(w, r)
	}))
	defer server.Close()

	client := NewClient("tablerip-test")
	client.BaseURL = server.URL

	filing, err := client.FindFiling("AAPL", "10-K", 2024)
	if err != nil {
		t.Fatalf("expected FindFiling to succeed: %v", err)
	}

	if filing.AccessionNumber != "0000320193-24-000106" {
		t.Errorf("unexpected accession number: %s", filing.AccessionNumber)
	}
	if filing.Year != 2024 {
		t.Errorf("expected year 2024, got %d", filing.Year)
	}

	// Test Download
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "filing.txt")
	filing.DocumentURL = server.URL + "/test-download.txt"

	if err := client.DownloadFiling(filing, outPath); err != nil {
		t.Fatalf("failed to download filing: %v", err)
	}

	content, err := os.ReadFile(outPath)
	if err != nil || string(content) != "MOCK SEC FILING DATA STREAM" {
		t.Fatalf("unexpected downloaded content: %s", string(content))
	}
}
