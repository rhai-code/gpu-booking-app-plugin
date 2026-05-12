package database

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

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

type GPUResourceSpec struct {
	Name          string  `json:"name"`
	Type          string  `json:"type"`
	Count         int     `json:"count"`
	Share         float64 `json:"share"`
	GPUEquivalent float64 `json:"gpuEquivalent"`
}

type GPUConfig struct {
	Resources   []GPUResourceSpec
	TotalCPU    int
	TotalMemory int
	FlavorName  string
}

var gpuConfig atomic.Pointer[GPUConfig]

func init() {
	gpuConfig.Store(&GPUConfig{
		Resources: []GPUResourceSpec{
			{Name: "Full GPU", Type: "nvidia.com/gpu", Count: 8, Share: 1.0, GPUEquivalent: 1.0},
		},
		TotalCPU:    64,
		TotalMemory: 256,
	})
}

func GetGPUConfig() *GPUConfig {
	return gpuConfig.Load()
}

func SetGPUConfig(cfg *GPUConfig) {
	gpuConfig.Store(cfg)
}

func FlavorName() string {
	if name := gpuConfig.Load().FlavorName; name != "" {
		return name
	}
	return "h200"
}

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

func IsGPUResource(name string) bool {
	for _, spec := range gpuConfig.Load().Resources {
		if spec.Type == name {
			return true
		}
	}
	return false
}

func GPUSpecByType(resType string) (GPUResourceSpec, bool) {
	for _, spec := range gpuConfig.Load().Resources {
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
	cfg := gpuConfig.Load()
	return Config{
		Resources:         cfg.Resources,
		BookingWindowDays: bookingWindowDays,
		TotalCPU:          cfg.TotalCPU,
		TotalMemory:       cfg.TotalMemory,
	}
}

type gpuConfigFile struct {
	Resources   []GPUResourceSpec `json:"resources"`
	TotalCPU    int               `json:"totalCpu"`
	TotalMemory int               `json:"totalMemory"`
}

func LoadConfigFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading gpu config: %w", err)
	}
	var fileCfg gpuConfigFile
	if err := json.Unmarshal(data, &fileCfg); err != nil {
		return fmt.Errorf("parsing gpu config: %w", err)
	}
	current := gpuConfig.Load()
	updated := &GPUConfig{
		Resources:   current.Resources,
		TotalCPU:    current.TotalCPU,
		TotalMemory: current.TotalMemory,
		FlavorName:  current.FlavorName,
	}
	if len(fileCfg.Resources) > 0 {
		updated.Resources = fileCfg.Resources
	}
	if fileCfg.TotalCPU > 0 {
		updated.TotalCPU = fileCfg.TotalCPU
	}
	if fileCfg.TotalMemory > 0 {
		updated.TotalMemory = fileCfg.TotalMemory
	}
	gpuConfig.Store(updated)
	return nil
}

func ScanBooking(rows *sql.Rows) (Booking, error) {
	var b Booking
	err := rows.Scan(&b.ID, &b.User, &b.Email, &b.Resource, &b.SlotIndex, &b.Date, &b.SlotType, &b.CreatedAt, &b.Source, &b.Description, &b.StartHour, &b.EndHour, &b.UtcOffset)
	return b, err
}

const BookingColumns = "id, user, email, resource, slot_index, date, slot_type, created_at, source, description, start_hour, end_hour, utc_offset"
