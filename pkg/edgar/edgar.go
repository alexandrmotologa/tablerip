package edgar

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Pre-indexed CIK cache for prominent public enterprise companies.
var topTickerCIK = map[string]int64{
	"AAPL":  320193,
	"MSFT":  789019,
	"NVDA":  1045810,
	"GOOGL": 1652044,
	"GOOG":  1652044,
	"AMZN":  1018724,
	"META":  1326801,
	"TSLA":  1318605,
	"BRK-A": 1067983,
	"BRK-B": 1067983,
	"JPM":   19617,
	"V":     1403161,
	"JNJ":   200406,
	"WMT":   104169,
	"PG":    80424,
	"XOM":   34088,
	"BAC":   70858,
	"ORCL":  1341439,
	"CSCO":  858877,
	"AMD":   2488,
	"INTC":  50863,
	"QCOM":  804328,
	"CRM":   1108524,
	"IBM":   51143,
}

// FilingMetadata represents an SEC EDGAR filing submission record.
type FilingMetadata struct {
	Ticker          string    `json:"ticker"`
	CIK             int64     `json:"cik"`
	Form            string    `json:"form"`
	AccessionNumber string    `json:"accession_number"`
	FilingDate      string    `json:"filing_date"`
	PrimaryDocument string    `json:"primary_document"`
	DocumentURL     string    `json:"document_url"`
	ReportDate      string    `json:"report_date"`
	Year            int       `json:"year"`
	Size            int64     `json:"size"`
	Timestamp       time.Time `json:"timestamp"`
}

// Client manages SEC EDGAR queries with strict SEC rate-limiting compliance.
type Client struct {
	HTTPClient *http.Client
	UserAgent  string
	BaseURL    string
}

// NewClient creates an SEC EDGAR Client with standard headers.
func NewClient(userAgent string) *Client {
	if userAgent == "" {
		userAgent = "tablerip/1.0 (quantitative-research@tablerip.org)"
	}
	return &Client{
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  userAgent,
		BaseURL:    "https://data.sec.gov",
	}
}

// LookupCIK resolves a stock ticker to its CIK number.
func (c *Client) LookupCIK(ticker string) (int64, error) {
	upper := strings.ToUpper(strings.TrimSpace(ticker))
	if cik, ok := topTickerCIK[upper]; ok {
		return cik, nil
	}

	// Fetch SEC company_tickers.json
	req, err := http.NewRequest("GET", "https://www.sec.gov/files/company_tickers.json", nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch SEC ticker directory: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("SEC API returned status %d", resp.StatusCode)
	}

	var tickerDir map[string]struct {
		CIK    int64  `json:"cik_str"`
		Ticker string `json:"ticker"`
		Title  string `json:"title"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tickerDir); err != nil {
		return 0, fmt.Errorf("failed to decode SEC ticker directory: %w", err)
	}

	for _, item := range tickerDir {
		if strings.EqualFold(item.Ticker, upper) {
			return item.CIK, nil
		}
	}

	return 0, fmt.Errorf("ticker '%s' not found in SEC EDGAR directory", upper)
}

type secSubmissions struct {
	CIK           string `json:"cik"`
	EntityName    string `json:"name"`
	FilingsRecent struct {
		AccessionNumber []string `json:"accessionNumber"`
		FilingDate      []string `json:"filingDate"`
		ReportDate      []string `json:"reportDate"`
		Form            []string `json:"form"`
		PrimaryDocument []string `json:"primaryDocument"`
		Size            []int64  `json:"size"`
	} `json:"filings"`
}

// FindFiling locates the latest or specific year filing (e.g. Form 10-K or 10-Q) for a ticker.
func (c *Client) FindFiling(ticker, form string, year int) (*FilingMetadata, error) {
	cik, err := c.LookupCIK(ticker)
	if err != nil {
		return nil, err
	}

	cikPadded := fmt.Sprintf("%010d", cik)
	url := fmt.Sprintf("%s/submissions/CIK%s.json", c.BaseURL, cikPadded)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to query SEC submissions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SEC submissions API returned status %d for CIK %d", resp.StatusCode, cik)
	}

	var sub secSubmissions
	if err := json.NewDecoder(resp.Body).Decode(&sub); err != nil {
		return nil, fmt.Errorf("failed to parse SEC submissions JSON: %w", err)
	}

	recent := sub.FilingsRecent
	targetForm := strings.ToUpper(strings.TrimSpace(form))

	for i := 0; i < len(recent.Form); i++ {
		currForm := recent.Form[i]
		if strings.EqualFold(currForm, targetForm) {
			fDate := recent.FilingDate[i]
			fYear := 0
			if len(fDate) >= 4 {
				fYear, _ = strconv.Atoi(fDate[:4])
			}

			if year <= 0 || fYear == year {
				accNum := recent.AccessionNumber[i]
				accNoHyphens := strings.ReplaceAll(accNum, "-", "")
				primaryDoc := recent.PrimaryDocument[i]
				docURL := fmt.Sprintf("https://www.sec.gov/Archives/edgar/data/%d/%s/%s",
					cik, accNoHyphens, primaryDoc)

				var docSize int64
				if i < len(recent.Size) {
					docSize = recent.Size[i]
				}

				return &FilingMetadata{
					Ticker:          strings.ToUpper(ticker),
					CIK:             cik,
					Form:            currForm,
					AccessionNumber: accNum,
					FilingDate:      fDate,
					PrimaryDocument: primaryDoc,
					DocumentURL:     docURL,
					ReportDate:      recent.ReportDate[i],
					Year:            fYear,
					Size:            docSize,
					Timestamp:       time.Now(),
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no matching filing found for %s (Form: %s, Year: %d)", ticker, form, year)
}

// DownloadFiling streams the filing document directly to disk.
func (c *Client) DownloadFiling(filing *FilingMetadata, outputPath string) error {
	if filing == nil || filing.DocumentURL == "" {
		return fmt.Errorf("invalid filing metadata")
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	req, err := http.NewRequest("GET", filing.DocumentURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download filing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with HTTP status %d", resp.StatusCode)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
