package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Booking struct {
	ID          string `json:"id"`
	User        string `json:"user"`
	Email       string `json:"email"`
	Resource    string `json:"resource"`
	SlotIndex   int    `json:"slotIndex"`
	Date        string `json:"date"`
	SlotType    string `json:"slotType"`
	CreatedAt   string `json:"createdAt"`
	Source      string `json:"source"`
	Description string `json:"description"`
	StartHour   int    `json:"startHour"`
	EndHour     int    `json:"endHour"`
	UtcOffset   int    `json:"utcOffset"`
}

// GPUResourceSpec defines a single GPU resource type with all its properties.
// This is the single source of truth for the GPU pool — used by the API config
// endpoint, reservation quota calculations, and Kueue sync filtering.
type GPUResourceSpec struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Count         int     `json:"count"`
	Share         float64 `json:"share"`
	GPUEquivalent float64 `json:"gpuEquivalent"`
}

var GPUResourceSpecs = []GPUResourceSpec{
	{Name: "H200 Full GPU", Type: "nvidia.com/gpu", Count: 8, Share: 0.0625, GPUEquivalent: 1.0},
	{Name: "MIG 3g.71gb", Type: "nvidia.com/mig-3g.71gb", Count: 8, Share: 0.03125, GPUEquivalent: 0.5},
	{Name: "MIG 2g.35gb", Type: "nvidia.com/mig-2g.35gb", Count: 8, Share: 0.015625, GPUEquivalent: 0.25},
	{Name: "MIG 1g.18gb", Type: "nvidia.com/mig-1g.18gb", Count: 16, Share: 0.0078125, GPUEquivalent: 0.125},
}

var (
	TotalCPU    = 316
	TotalMemory = 3460 // Gi
)

const (
	// Booking sources
	SourceReserved = "reserved"
	SourceConsumed = "consumed"

	// Slot types
	SlotTypeFull = "full"
)

type Config struct {
	Resources         []GPUResourceSpec `json:"resources"`
	BookingWindowDays int               `json:"bookingWindowDays"`
	TotalCPU          int               `json:"totalCpu"`
	TotalMemory       int               `json:"totalMemory"`
}

// IsGPUResource returns true if the resource name is a known GPU resource type.
func IsGPUResource(name string) bool {
	for _, spec := range GPUResourceSpecs {
		if spec.Type == name {
			return true
		}
	}
	return false
}

// GPUSpecByType returns the spec for a GPU resource type, or ok=false.
func GPUSpecByType(resType string) (GPUResourceSpec, bool) {
	for _, spec := range GPUResourceSpecs {
		if spec.Type == resType {
			return spec, true
		}
	}
	return GPUResourceSpec{}, false
}

var (
	db         *sql.DB
	DBMu       sync.Mutex
	DBFilePath string
)

func DB() *sql.DB {
	return db
}

func Init(dbPath string) error {
	DBFilePath = dbPath

	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating db directory: %w", err)
	}

	return OpenDB(dbPath)
}

func OpenDB(dbPath string) error {
	var err error
	db, err = sql.Open("sqlite3", dbPath+"?_journal_mode=WAL")
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS bookings (
			id TEXT PRIMARY KEY,
			user TEXT NOT NULL,
			email TEXT NOT NULL DEFAULT '',
			resource TEXT NOT NULL,
			slot_index INTEGER NOT NULL,
			date TEXT NOT NULL,
			slot_type TEXT NOT NULL,
			created_at TEXT NOT NULL,
			source TEXT NOT NULL DEFAULT 'reserved',
			description TEXT NOT NULL DEFAULT '',
			start_hour INTEGER NOT NULL DEFAULT 0,
			end_hour INTEGER NOT NULL DEFAULT 24,
			utc_offset INTEGER NOT NULL DEFAULT 0,
			UNIQUE(resource, slot_index, date, slot_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating table: %w", err)
	}

	// Indexes for query performance
	db.Exec("CREATE INDEX IF NOT EXISTS idx_bookings_date ON bookings(date)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_bookings_resource_date ON bookings(resource, date)")
	db.Exec("CREATE INDEX IF NOT EXISTS idx_bookings_source ON bookings(source)")

	// Migrations: add columns if missing (existing databases)
	db.Exec("ALTER TABLE bookings ADD COLUMN source TEXT NOT NULL DEFAULT 'reserved'")
	db.Exec("ALTER TABLE bookings ADD COLUMN description TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE bookings ADD COLUMN start_hour INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE bookings ADD COLUMN end_hour INTEGER NOT NULL DEFAULT 24")
	db.Exec("ALTER TABLE bookings ADD COLUMN utc_offset INTEGER NOT NULL DEFAULT 0")

	return nil
}

func Close() {
	if db != nil {
		db.Close()
	}
}

func GetConfig(bookingWindowDays int) Config {
	return Config{
		Resources:         GPUResourceSpecs,
		BookingWindowDays: bookingWindowDays,
		TotalCPU:          TotalCPU,
		TotalMemory:       TotalMemory,
	}
}

// gpuConfigFile is the JSON structure for the external GPU config file.
type gpuConfigFile struct {
	Resources   []GPUResourceSpec `json:"resources"`
	TotalCPU    int               `json:"totalCpu"`
	TotalMemory int               `json:"totalMemory"`
}

// LoadConfigFromFile reads GPU configuration from a JSON file, overwriting
// the built-in defaults. Returns an error if the file cannot be read or parsed.
func LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading gpu config: %w", err)
	}
	var cfg gpuConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parsing gpu config: %w", err)
	}
	if len(cfg.Resources) > 0 {
		GPUResourceSpecs = cfg.Resources
	}
	if cfg.TotalCPU > 0 {
		TotalCPU = cfg.TotalCPU
	}
	if cfg.TotalMemory > 0 {
		TotalMemory = cfg.TotalMemory
	}
	return nil
}

func ScanBooking(rows *sql.Rows) (Booking, error) {
	var b Booking
	err := rows.Scan(&b.ID, &b.User, &b.Email, &b.Resource, &b.SlotIndex, &b.Date, &b.SlotType, &b.CreatedAt, &b.Source, &b.Description, &b.StartHour, &b.EndHour, &b.UtcOffset)
	return b, err
}

const BookingColumns = "id, user, email, resource, slot_index, date, slot_type, created_at, source, description, start_hour, end_hour, utc_offset"
