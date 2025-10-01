package dmr

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

// Sample DMR ID data for testing
const sampleCSV = `DMRID,Callsign,Name,City,State,Country,Remarks
1234567,W1ABC,John Doe,Boston,MA,United States,Test user 1
2345678,K2XYZ,Jane Smith,New York,NY,United States,Test user 2
3456789,N3QRS,Bob Johnson,Philadelphia,PA,United States,Test user 3
`

const sampleDAT = `1234567 W1ABC John_Doe Boston MA
2345678 K2XYZ Jane_Smith New_York NY
3456789 N3QRS Bob_Johnson Philadelphia PA
`

func TestNewLookupWithCSV(t *testing.T) {
	// Create temp CSV file
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")

	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	config := LookupConfig{
		FilePath:     csvFile,
		AutoDownload: false,
	}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	if lookup.Count() != 3 {
		t.Errorf("Expected 3 entries, got %d", lookup.Count())
	}
}

func TestNewLookupWithDAT(t *testing.T) {
	// Create temp DAT file
	tmpDir := t.TempDir()
	datFile := filepath.Join(tmpDir, "test.dat")

	if err := os.WriteFile(datFile, []byte(sampleDAT), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})

	config := LookupConfig{
		FilePath:     datFile,
		AutoDownload: false,
	}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	if lookup.Count() != 3 {
		t.Errorf("Expected 3 entries, got %d", lookup.Count())
	}
}

func TestGetCallsign(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	tests := []struct {
		dmrID    uint32
		expected string
	}{
		{1234567, "W1ABC"},
		{2345678, "K2XYZ"},
		{3456789, "N3QRS"},
		{9999999, ""}, // Not found
	}

	for _, test := range tests {
		callsign := lookup.GetCallsign(test.dmrID)
		if callsign != test.expected {
			t.Errorf("GetCallsign(%d) = %s, expected %s", test.dmrID, callsign, test.expected)
		}
	}
}

func TestGetDMRID(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	tests := []struct {
		callsign   string
		expectedID uint32
		expectedOK bool
	}{
		{"W1ABC", 1234567, true},
		{"w1abc", 1234567, true}, // Case insensitive
		{"K2XYZ", 2345678, true},
		{"N3QRS", 3456789, true},
		{"NOTFOUND", 0, false},
	}

	for _, test := range tests {
		dmrID, ok := lookup.GetDMRID(test.callsign)
		if ok != test.expectedOK {
			t.Errorf("GetDMRID(%s) ok = %v, expected %v", test.callsign, ok, test.expectedOK)
		}
		if ok && dmrID != test.expectedID {
			t.Errorf("GetDMRID(%s) = %d, expected %d", test.callsign, dmrID, test.expectedID)
		}
	}
}

func TestGetEntry(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	entry, ok := lookup.GetEntry(1234567)
	if !ok {
		t.Fatal("Expected to find entry for 1234567")
	}

	if entry.Callsign != "W1ABC" {
		t.Errorf("Expected callsign W1ABC, got %s", entry.Callsign)
	}

	if entry.Name != "John Doe" {
		t.Errorf("Expected name John Doe, got %s", entry.Name)
	}

	if entry.City != "Boston" {
		t.Errorf("Expected city Boston, got %s", entry.City)
	}

	if entry.State != "MA" {
		t.Errorf("Expected state MA, got %s", entry.State)
	}

	if entry.Country != "United States" {
		t.Errorf("Expected country United States, got %s", entry.Country)
	}
}

func TestParseCSVWithComments(t *testing.T) {
	csvWithHeader := `# Comment line
DMRID,Callsign,Name,City,State,Country
1234567,W1ABC,John Doe,Boston,MA,United States
# Another comment
2345678,K2XYZ,Jane Smith,New York,NY,United States
`

	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(csvWithHeader), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	// Should have 2 entries (comments and header skipped)
	if lookup.Count() != 2 {
		t.Errorf("Expected 2 entries, got %d", lookup.Count())
	}
}

func TestParseInvalidData(t *testing.T) {
	invalidCSV := `DMRID,Callsign,Name
INVALID,W1ABC,John Doe
1234567,K2XYZ,Jane Smith
`

	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(invalidCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	// Should have 1 valid entry (invalid line skipped)
	if lookup.Count() != 1 {
		t.Errorf("Expected 1 entry, got %d", lookup.Count())
	}

	// Valid entry should be accessible
	if callsign := lookup.GetCallsign(1234567); callsign != "K2XYZ" {
		t.Errorf("Expected K2XYZ, got %s", callsign)
	}
}

func TestRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")

	// Write initial data
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	if lookup.Count() != 3 {
		t.Errorf("Expected 3 entries initially, got %d", lookup.Count())
	}

	// Update file with new data
	newCSV := `DMRID,Callsign,Name,City,State,Country
1234567,W1ABC,John Doe,Boston,MA,United States
2345678,K2XYZ,Jane Smith,New York,NY,United States
3456789,N3QRS,Bob Johnson,Philadelphia,PA,United States
4567890,W4DEF,Alice Brown,Atlanta,GA,United States
`
	if err := os.WriteFile(csvFile, []byte(newCSV), 0644); err != nil {
		t.Fatalf("Failed to write updated test file: %v", err)
	}

	// Refresh
	if err := lookup.Refresh(); err != nil {
		t.Fatalf("Failed to refresh: %v", err)
	}

	if lookup.Count() != 4 {
		t.Errorf("Expected 4 entries after refresh, got %d", lookup.Count())
	}

	// Verify new entry exists
	if callsign := lookup.GetCallsign(4567890); callsign != "W4DEF" {
		t.Errorf("Expected W4DEF, got %s", callsign)
	}
}

func TestLastRefresh(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	before := time.Now()
	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}
	after := time.Now()

	lastRefresh := lookup.LastRefresh()
	if lastRefresh.Before(before) || lastRefresh.After(after) {
		t.Errorf("LastRefresh time %v not in expected range [%v, %v]", lastRefresh, before, after)
	}
}

func TestCount(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	if lookup.Count() != 3 {
		t.Errorf("Expected count 3, got %d", lookup.Count())
	}
}

func TestEmptyDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "empty.csv")
	if err := os.WriteFile(csvFile, []byte("DMRID,Callsign,Name\n"), 0644); err != nil {
		t.Fatalf("Failed to create empty test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	if lookup.Count() != 0 {
		t.Errorf("Expected count 0, got %d", lookup.Count())
	}

	// Lookups should return empty/false
	if callsign := lookup.GetCallsign(1234567); callsign != "" {
		t.Errorf("Expected empty callsign, got %s", callsign)
	}

	if _, ok := lookup.GetDMRID("W1ABC"); ok {
		t.Error("Expected GetDMRID to return false for empty database")
	}
}

func TestConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, err := NewLookup(config, log)
	if err != nil {
		t.Fatalf("Failed to create lookup: %v", err)
	}

	// Run concurrent lookups
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				lookup.GetCallsign(1234567)
				lookup.GetDMRID("W1ABC")
				lookup.Count()
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkGetCallsign(b *testing.B) {
	tmpDir := b.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		b.Fatalf("Failed to create benchmark test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, _ := NewLookup(config, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookup.GetCallsign(1234567)
	}
}

func BenchmarkGetDMRID(b *testing.B) {
	tmpDir := b.TempDir()
	csvFile := filepath.Join(tmpDir, "test.csv")
	if err := os.WriteFile(csvFile, []byte(sampleCSV), 0644); err != nil {
		b.Fatalf("Failed to create test file: %v", err)
	}

	log, _ := logger.New(logger.Config{Level: "error", Format: "text"})
	config := LookupConfig{FilePath: csvFile}

	lookup, _ := NewLookup(config, log)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lookup.GetDMRID("W1ABC")
	}
}
