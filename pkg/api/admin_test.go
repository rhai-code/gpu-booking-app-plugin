package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func TestAdminListBookings(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	for i := 0; i < 3; i++ {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			"booking-"+string(rune('a'+i)), "alice", "", "nvidia.com/gpu", i, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin?limit=10&offset=0", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminListBookings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
		Total    int                `json:"total"`
		Limit    int                `json:"limit"`
		Offset   int                `json:"offset"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 3 {
		t.Errorf("total = %d, want 3", resp.Total)
	}
	if len(resp.Bookings) != 3 {
		t.Errorf("bookings count = %d, want 3", len(resp.Bookings))
	}
}

func TestAdminListBookingsNonAdmin(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	AdminListBookings(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAdminListBookingsWithFilters(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "alice", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-2", "bob", "", "nvidia.com/mig-1g.18gb", 0, today, "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)

	// Filter by source
	req := httptest.NewRequest(http.MethodGet, "/admin?source=consumed", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()
	AdminListBookings(w, req)

	var resp struct {
		Total int `json:"total"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("filtered total = %d, want 1", resp.Total)
	}

	// Filter by resource
	req = httptest.NewRequest(http.MethodGet, "/admin?resource=nvidia.com/gpu", nil)
	req = reqWithUser(req, testAdmin())
	w = httptest.NewRecorder()
	AdminListBookings(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("resource filtered total = %d, want 1", resp.Total)
	}

	// Search by user
	req = httptest.NewRequest(http.MethodGet, "/admin?search=alice", nil)
	req = reqWithUser(req, testAdmin())
	w = httptest.NewRecorder()
	AdminListBookings(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Total != 1 {
		t.Errorf("search filtered total = %d, want 1", resp.Total)
	}
}

func TestAdminListBookingsPagination(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	for i := 0; i < 5; i++ {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			"booking-"+string(rune('a'+i)), "alice", "", "nvidia.com/gpu", i, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin?limit=2&offset=0", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()
	AdminListBookings(w, req)

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
		Total    int                `json:"total"`
		Limit    int                `json:"limit"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 2 {
		t.Errorf("page size = %d, want 2", len(resp.Bookings))
	}
	if resp.Total != 5 {
		t.Errorf("total = %d, want 5", resp.Total)
	}
	if resp.Limit != 2 {
		t.Errorf("limit = %d, want 2", resp.Limit)
	}
}

func TestAdminDeleteBooking(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "alice", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	req := httptest.NewRequest(http.MethodDelete, "/admin?id=booking-1", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminDeleteBooking(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM bookings").Scan(&count)
	if count != 0 {
		t.Errorf("remaining bookings = %d, want 0", count)
	}
}

func TestAdminDeleteBookingNotFound(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin?id=booking-999", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminDeleteBooking(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestAdminDeleteBookingNonAdmin(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin?id=booking-1", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	AdminDeleteBooking(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAdminDeleteAllBookings(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	for i := 0; i < 3; i++ {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			"booking-"+string(rune('a'+i)), "alice", "", "nvidia.com/gpu", i, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	// No id parameter = delete all
	req := httptest.NewRequest(http.MethodDelete, "/admin", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminDeleteBooking(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Count int64 `json:"count"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Count != 3 {
		t.Errorf("deleted count = %d, want 3", resp.Count)
	}
}

func TestAdminReservationToggle(t *testing.T) {
	setupTestDB(t)

	body := `{"enabled":false}`
	req := httptest.NewRequest(http.MethodPost, "/admin/reservations", strings.NewReader(body))
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminReservationToggleHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp struct {
		ReservationSyncEnabled bool `json:"reservationSyncEnabled"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.ReservationSyncEnabled != false {
		t.Error("expected reservationSyncEnabled = false")
	}
}

func TestAdminReservationToggleNonAdmin(t *testing.T) {
	setupTestDB(t)

	body := `{"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/admin/reservations", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	AdminReservationToggleHandler(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAdminReservationToggleInvalidJSON(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/reservations", strings.NewReader("{bad"))
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminReservationToggleHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAdminDeleteBookingInvalidID(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/admin?id=INVALID-FORMAT", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminDeleteBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestAdminListBookingsLimitClamping(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/admin?limit=9999", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminListBookings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Limit int `json:"limit"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Limit != 1000 {
		t.Errorf("limit = %d, want 1000 (clamped)", resp.Limit)
	}
}

func TestAdminExportDatabase(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/database/export", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminExportDatabase(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Disposition"); !strings.Contains(ct, "bookings.db") {
		t.Errorf("Content-Disposition = %q, want bookings.db", ct)
	}
	if w.Body.Len() == 0 {
		t.Error("expected non-empty body")
	}
}

func TestAdminExportDatabaseNonAdmin(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/database/export", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	AdminExportDatabase(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAdminImportDatabase(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-before", "alice", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	// Export the database
	exportReq := httptest.NewRequest(http.MethodGet, "/admin/database/export", nil)
	exportReq = reqWithUser(exportReq, testAdmin())
	exportW := httptest.NewRecorder()
	AdminExportDatabase(exportW, exportReq)

	if exportW.Code != http.StatusOK {
		t.Fatalf("export status = %d", exportW.Code)
	}
	dbBytes := exportW.Body.Bytes()

	// Re-import it
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("database", "bookings.db")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := io.Copy(part, bytes.NewReader(dbBytes)); err != nil {
		t.Fatalf("copy: %v", err)
	}
	writer.Close()

	importReq := httptest.NewRequest(http.MethodPost, "/admin/database/import", &body)
	importReq.Header.Set("Content-Type", writer.FormDataContentType())
	importReq = reqWithUser(importReq, testAdmin())
	importW := httptest.NewRecorder()

	AdminImportDatabase(importW, importReq)

	if importW.Code != http.StatusOK {
		t.Fatalf("import status = %d; body = %s", importW.Code, importW.Body.String())
	}

	var resp map[string]string
	json.NewDecoder(importW.Body).Decode(&resp)
	if resp["status"] != "imported" {
		t.Errorf("status = %q, want imported", resp["status"])
	}

	var count int
	database.DB().QueryRow("SELECT COUNT(*) FROM bookings WHERE id = 'booking-before'").Scan(&count)
	if count != 1 {
		t.Errorf("booking count after import = %d, want 1", count)
	}
}

func TestAdminImportDatabaseNonAdmin(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/database/import", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	AdminImportDatabase(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestAdminImportDatabaseMissingField(t *testing.T) {
	setupTestDB(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("wrong_field", "data")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/admin/database/import", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminImportDatabase(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusBadRequest, w.Body.String())
	}
}

func TestAdminListBookingsDefaultPagination(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	AdminListBookings(w, req)

	var resp struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Limit != 100 {
		t.Errorf("default limit = %d, want 100", resp.Limit)
	}
	if resp.Offset != 0 {
		t.Errorf("default offset = %d, want 0", resp.Offset)
	}
}

func TestAdminListBookingsWithOffset(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	for i := range 5 {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			fmt.Sprintf("booking-%d", i), "alice", "", "nvidia.com/gpu", i, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin?limit=2&offset=3", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()
	AdminListBookings(w, req)

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
		Total    int                `json:"total"`
		Offset   int                `json:"offset"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 2 {
		t.Errorf("page size = %d, want 2", len(resp.Bookings))
	}
	if resp.Offset != 3 {
		t.Errorf("offset = %d, want 3", resp.Offset)
	}
}

func TestAdminListBookingsServerSort(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "charlie", "", "nvidia.com/gpu", 0, "2026-01-01", "full", "2026-01-01T10:00:00Z", database.SourceReserved, "", 0, 24, 0,
	)
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-2", "alice", "", "nvidia.com/gpu", 1, "2026-03-01", "full", "2026-02-01T10:00:00Z", database.SourceReserved, "", 0, 24, 0,
	)
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-3", "bob", "", "nvidia.com/gpu", 2, "2026-02-01", "full", "2026-03-01T10:00:00Z", database.SourceReserved, "", 0, 24, 0,
	)

	// Sort by date descending
	req := httptest.NewRequest(http.MethodGet, "/admin?sort=date&sort_dir=desc", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()
	AdminListBookings(w, req)

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 3 {
		t.Fatalf("count = %d, want 3", len(resp.Bookings))
	}
	if resp.Bookings[0].Date != "2026-03-01" {
		t.Errorf("first booking date = %s, want 2026-03-01", resp.Bookings[0].Date)
	}
	if resp.Bookings[2].Date != "2026-01-01" {
		t.Errorf("last booking date = %s, want 2026-01-01", resp.Bookings[2].Date)
	}

	// Sort by user ascending
	req = httptest.NewRequest(http.MethodGet, "/admin?sort=user&sort_dir=asc", nil)
	req = reqWithUser(req, testAdmin())
	w = httptest.NewRecorder()
	AdminListBookings(w, req)

	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Bookings[0].User != "alice" {
		t.Errorf("first user = %s, want alice", resp.Bookings[0].User)
	}
	if resp.Bookings[2].User != "charlie" {
		t.Errorf("last user = %s, want charlie", resp.Bookings[2].User)
	}

	// Invalid sort key should fall back to date
	req = httptest.NewRequest(http.MethodGet, "/admin?sort=invalid_column&sort_dir=desc", nil)
	req = reqWithUser(req, testAdmin())
	w = httptest.NewRecorder()
	AdminListBookings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("invalid sort key should not error, got status %d", w.Code)
	}
}
