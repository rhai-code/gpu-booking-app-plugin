package database

import (
	"database/sql"
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

const (
	TotalCPU    = 316
	TotalMemory = 3460 // Gi
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
			UNIQUE(resource, slot_index, date, slot_type)
		)
	`)
	if err != nil {
		return fmt.Errorf("creating table: %w", err)
	}

	// Migrations: add columns if missing (existing databases)
	db.Exec("ALTER TABLE bookings ADD COLUMN source TEXT NOT NULL DEFAULT 'reserved'")
	db.Exec("ALTER TABLE bookings ADD COLUMN description TEXT NOT NULL DEFAULT ''")
	db.Exec("ALTER TABLE bookings ADD COLUMN start_hour INTEGER NOT NULL DEFAULT 0")
	db.Exec("ALTER TABLE bookings ADD COLUMN end_hour INTEGER NOT NULL DEFAULT 24")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS llm_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			model_name TEXT NOT NULL DEFAULT '',
			provider_type TEXT NOT NULL DEFAULT 'openai-compatible',
			owner TEXT NOT NULL,
			is_global INTEGER NOT NULL DEFAULT 0,
			enabled INTEGER NOT NULL DEFAULT 1,
			created_at TEXT NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("creating llm_endpoints table: %w", err)
	}

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

func ScanBooking(rows *sql.Rows) (Booking, error) {
	var b Booking
	err := rows.Scan(&b.ID, &b.User, &b.Email, &b.Resource, &b.SlotIndex, &b.Date, &b.SlotType, &b.CreatedAt, &b.Source, &b.Description, &b.StartHour, &b.EndHour)
	return b, err
}

const BookingColumns = "id, user, email, resource, slot_index, date, slot_type, created_at, source, description, start_hour, end_hour"

type LLMEndpoint struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	APIKey       string `json:"api_key,omitempty"`
	ModelName    string `json:"model_name"`
	ProviderType string `json:"provider_type"`
	Owner        string `json:"owner"`
	IsGlobal     bool   `json:"is_global"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

const LLMEndpointColumns = "id, name, url, api_key, model_name, provider_type, owner, is_global, enabled, created_at"

func ScanLLMEndpoint(rows *sql.Rows) (LLMEndpoint, error) {
	var e LLMEndpoint
	var isGlobal, enabled int
	err := rows.Scan(&e.ID, &e.Name, &e.URL, &e.APIKey, &e.ModelName, &e.ProviderType, &e.Owner, &isGlobal, &enabled, &e.CreatedAt)
	e.IsGlobal = isGlobal == 1
	e.Enabled = enabled == 1
	return e, err
}
