package dmr

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// LookupEntry represents a DMR ID database entry
type LookupEntry struct {
	DMRID     uint32
	Callsign  string
	Name      string
	City      string
	State     string
	Country   string
	Remarks   string
	UpdatedAt time.Time
}

// Lookup provides DMR ID ↔ Callsign lookups
type Lookup struct {
	mu sync.RWMutex

	// Bidirectional maps
	dmrToEntry map[uint32]*LookupEntry
	callToDMR  map[string]uint32

	// Configuration
	filePath     string
	downloadURL  string
	autoDownload bool
	lastRefresh  time.Time

	// Logger
	logger *logger.Logger
}

// LookupConfig holds lookup configuration
type LookupConfig struct {
	FilePath        string        // Path to local DMR ID file
	DownloadURL     string        // URL to download database from
	AutoDownload    bool          // Auto-download if file doesn't exist
	RefreshInterval time.Duration // How often to refresh (0 = manual only)
}

// NewLookup creates a new DMR ID lookup service
func NewLookup(config LookupConfig, log *logger.Logger) (*Lookup, error) {
	l := &Lookup{
		dmrToEntry:   make(map[uint32]*LookupEntry),
		callToDMR:    make(map[string]uint32),
		filePath:     config.FilePath,
		downloadURL:  config.DownloadURL,
		autoDownload: config.AutoDownload,
		logger:       log.WithComponent("dmr-lookup"),
	}

	// Try to load from file first
	if config.FilePath != "" {
		if err := l.LoadFromFile(config.FilePath); err != nil {
			l.logger.Warn("Failed to load DMR ID database from file", logger.Error(err))

			// If auto-download is enabled and we have a URL, try downloading
			if config.AutoDownload && config.DownloadURL != "" {
				l.logger.Info("Attempting to download DMR ID database")
				if err := l.Download(config.DownloadURL, config.FilePath); err != nil {
					return nil, fmt.Errorf("failed to download DMR ID database: %w", err)
				}

				// Try loading again
				if err := l.LoadFromFile(config.FilePath); err != nil {
					return nil, fmt.Errorf("failed to load downloaded database: %w", err)
				}
			} else {
				return nil, fmt.Errorf("failed to load DMR ID database: %w", err)
			}
		}
	}

	l.logger.Info("DMR ID lookup initialized",
		logger.Int("entries", len(l.dmrToEntry)))

	return l, nil
}

// LoadFromFile loads DMR IDs from a local file
// Supports multiple formats:
// - DMRIds.dat: space-separated (ID CALLSIGN NAME CITY STATE)
// - CSV: comma-separated (ID,CALLSIGN,NAME,CITY,STATE,COUNTRY)
func (l *Lookup) LoadFromFile(filepath string) error {
	l.logger.Info("Loading DMR ID database from file", logger.String("path", filepath))

	file, err := os.Open(filepath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			l.logger.Warn("Failed to close file", logger.Error(err))
		}
	}()

	// Detect format by extension or content
	isCSV := strings.HasSuffix(filepath, ".csv")

	var count int
	var parseErr error

	if isCSV {
		count, parseErr = l.parseCSV(file)
	} else {
		count, parseErr = l.parseDat(file)
	}

	if parseErr != nil {
		return parseErr
	}

	l.mu.Lock()
	l.lastRefresh = time.Now()
	l.mu.Unlock()

	l.logger.Info("DMR ID database loaded successfully",
		logger.Int("entries", count),
		logger.String("format", map[bool]string{true: "CSV", false: "DAT"}[isCSV]))

	return nil
}

// parseCSV parses CSV format database
// Expected columns: DMRID,Callsign,Name,City,State,Country,Remarks
func (l *Lookup) parseCSV(r io.Reader) (int, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1 // Variable number of fields
	reader.TrimLeadingSpace = true

	count := 0
	lineNum := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return count, fmt.Errorf("CSV parse error at line %d: %w", lineNum, err)
		}

		lineNum++

		// Skip header row
		if lineNum == 1 && !isNumeric(record[0]) {
			continue
		}

		// Need at least DMRID and Callsign
		if len(record) < 2 {
			continue
		}

		// Parse DMR ID
		dmrid, err := strconv.ParseUint(record[0], 10, 32)
		if err != nil {
			l.logger.Debug("Skipping invalid DMR ID",
				logger.String("value", record[0]),
				logger.Int("line", lineNum))
			continue
		}

		entry := &LookupEntry{
			DMRID:     uint32(dmrid),
			Callsign:  strings.TrimSpace(record[1]),
			UpdatedAt: time.Now(),
		}

		// Optional fields
		if len(record) > 2 {
			entry.Name = strings.TrimSpace(record[2])
		}
		if len(record) > 3 {
			entry.City = strings.TrimSpace(record[3])
		}
		if len(record) > 4 {
			entry.State = strings.TrimSpace(record[4])
		}
		if len(record) > 5 {
			entry.Country = strings.TrimSpace(record[5])
		}
		if len(record) > 6 {
			entry.Remarks = strings.TrimSpace(record[6])
		}

		l.addEntry(entry)
		count++
	}

	return count, nil
}

// parseDat parses DMRIds.dat format (space/tab separated)
// Format: DMRID CALLSIGN NAME CITY STATE
func (l *Lookup) parseDat(r io.Reader) (int, error) {
	scanner := bufio.NewScanner(r)
	count := 0
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split on whitespace
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		// Parse DMR ID
		dmrid, err := strconv.ParseUint(fields[0], 10, 32)
		if err != nil {
			l.logger.Debug("Skipping invalid DMR ID",
				logger.String("value", fields[0]),
				logger.Int("line", lineNum))
			continue
		}

		entry := &LookupEntry{
			DMRID:     uint32(dmrid),
			Callsign:  fields[1],
			UpdatedAt: time.Now(),
		}

		// Optional fields
		if len(fields) > 2 {
			entry.Name = fields[2]
		}
		if len(fields) > 3 {
			entry.City = fields[3]
		}
		if len(fields) > 4 {
			entry.State = fields[4]
		}

		l.addEntry(entry)
		count++
	}

	if err := scanner.Err(); err != nil {
		return count, fmt.Errorf("scanner error: %w", err)
	}

	return count, nil
}

// addEntry adds an entry to both maps (thread-safe internally)
func (l *Lookup) addEntry(entry *LookupEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.dmrToEntry[entry.DMRID] = entry

	// Normalize callsign (uppercase)
	normalizedCall := strings.ToUpper(entry.Callsign)
	l.callToDMR[normalizedCall] = entry.DMRID
}

// GetCallsign looks up a callsign by DMR ID
func (l *Lookup) GetCallsign(dmrID uint32) string {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if entry, ok := l.dmrToEntry[dmrID]; ok {
		return entry.Callsign
	}

	return ""
}

// GetDMRID looks up a DMR ID by callsign
func (l *Lookup) GetDMRID(callsign string) (uint32, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	normalizedCall := strings.ToUpper(callsign)
	dmrid, ok := l.callToDMR[normalizedCall]
	return dmrid, ok
}

// GetEntry gets full entry by DMR ID
func (l *Lookup) GetEntry(dmrID uint32) (*LookupEntry, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, ok := l.dmrToEntry[dmrID]
	return entry, ok
}

// Count returns the number of entries in the database
func (l *Lookup) Count() int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return len(l.dmrToEntry)
}

// LastRefresh returns when the database was last refreshed
func (l *Lookup) LastRefresh() time.Time {
	l.mu.RLock()
	defer l.mu.RUnlock()

	return l.lastRefresh
}

// Download downloads the DMR ID database from a URL
func (l *Lookup) Download(url, savePath string) error {
	l.logger.Info("Downloading DMR ID database", logger.String("url", url))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	// Download
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			l.logger.Warn("Failed to close response body", logger.Error(err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP status %d: %s", resp.StatusCode, resp.Status)
	}

	// Create output file
	out, err := os.Create(savePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			l.logger.Warn("Failed to close output file", logger.Error(err))
		}
	}()

	// Copy data
	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	l.logger.Info("DMR ID database downloaded",
		logger.Int64("bytes", written),
		logger.String("path", savePath))

	return nil
}

// Refresh reloads the database from file
func (l *Lookup) Refresh() error {
	if l.filePath == "" {
		return fmt.Errorf("no file path configured")
	}

	// Clear existing data
	l.mu.Lock()
	l.dmrToEntry = make(map[uint32]*LookupEntry)
	l.callToDMR = make(map[string]uint32)
	l.mu.Unlock()

	// Reload
	return l.LoadFromFile(l.filePath)
}

// StartAutoRefresh starts automatic periodic refresh
func (l *Lookup) StartAutoRefresh(interval time.Duration, stopChan <-chan struct{}) {
	if interval <= 0 {
		return
	}

	l.logger.Info("Starting auto-refresh", logger.Duration("interval", interval))

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopChan:
			l.logger.Info("Auto-refresh stopped")
			return
		case <-ticker.C:
			l.logger.Info("Auto-refreshing DMR ID database")

			// Download fresh copy if auto-download is enabled
			if l.autoDownload && l.downloadURL != "" {
				if err := l.Download(l.downloadURL, l.filePath); err != nil {
					l.logger.Error("Failed to download database", logger.Error(err))
					continue
				}
			}

			// Refresh from file
			if err := l.Refresh(); err != nil {
				l.logger.Error("Failed to refresh database", logger.Error(err))
			}
		}
	}
}

// Helper functions

func isNumeric(s string) bool {
	_, err := strconv.ParseUint(s, 10, 64)
	return err == nil
}
