package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func TestCreateBooking(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"full","description":"test","startHour":0,"endHour":24}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var b database.Booking
	if err := json.NewDecoder(w.Body).Decode(&b); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if b.User != "testuser" {
		t.Errorf("User = %q, want testuser", b.User)
	}
	if b.Resource != "nvidia.com/gpu" {
		t.Errorf("Resource = %q, want nvidia.com/gpu", b.Resource)
	}
	if b.Source != database.SourceReserved {
		t.Errorf("Source = %q, want reserved", b.Source)
	}
}

func TestCreateBookingInvalidResource(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"fake/gpu","slotIndex":0,"date":"` + date + `","slotType":"full"}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBookingInvalidSlotType(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"partial"}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBookingInvalidSlotIndex(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"nvidia.com/gpu","slotIndex":99,"date":"` + date + `","slotType":"full"}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBookingInvalidDate(t *testing.T) {
	setupTestDB(t)

	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"2020-01-01","slotType":"full"}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestCreateBookingConflict(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"full"}`

	// First booking
	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()
	CreateBooking(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("first booking: status = %d", w.Code)
	}

	// Duplicate should conflict
	req = httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w = httptest.NewRecorder()
	CreateBooking(w, req)
	if w.Code != http.StatusConflict {
		t.Errorf("duplicate: status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestCreateBookingDescriptionTruncation(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	longDesc := strings.Repeat("x", 200)
	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"full","description":"` + longDesc + `"}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}

	var b database.Booking
	json.NewDecoder(w.Body).Decode(&b)
	if len(b.Description) != 160 {
		t.Errorf("description length = %d, want 160", len(b.Description))
	}
}

func TestCreateBookingHourClamping(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"full","startHour":-5,"endHour":99}`

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}

	var b database.Booking
	json.NewDecoder(w.Body).Decode(&b)
	if b.StartHour != 0 {
		t.Errorf("StartHour = %d, want 0", b.StartHour)
	}
	if b.EndHour != 24 {
		t.Errorf("EndHour = %d, want 24", b.EndHour)
	}
}

func TestCreateBookingEvictsConsumed(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	// Insert a consumed booking directly
	_, err := db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"consumed-1", "kueue-user", "", "nvidia.com/gpu", 0, date, "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)
	if err != nil {
		t.Fatalf("insert consumed: %v", err)
	}

	body := `{"resource":"nvidia.com/gpu","slotIndex":0,"date":"` + date + `","slotType":"full"}`
	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	// Consumed booking should be gone
	var count int
	db.QueryRow("SELECT COUNT(*) FROM bookings WHERE id = 'consumed-1'").Scan(&count)
	if count != 0 {
		t.Error("consumed booking should have been evicted")
	}
}

func TestCreateBookingInvalidJSON(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/bookings", strings.NewReader("{bad"))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	CreateBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestGetBookings(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().UTC().Format("2006-01-02")
	_, err := db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "testuser", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "desc", 0, 24, 0,
	)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/bookings", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	GetBookings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Bookings           []database.Booking `json:"bookings"`
		ActiveReservations map[string]string  `json:"activeReservations"`
		CurrentUser        string             `json:"currentUser"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Bookings) != 1 {
		t.Errorf("bookings count = %d, want 1", len(resp.Bookings))
	}
	if resp.CurrentUser != "testuser" {
		t.Errorf("currentUser = %q, want testuser", resp.CurrentUser)
	}
	if _, ok := resp.ActiveReservations["testuser"]; !ok {
		t.Error("expected testuser in activeReservations")
	}
}

func TestGetBookingsEmpty(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodGet, "/bookings", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	GetBookings(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 0 {
		t.Errorf("bookings count = %d, want 0", len(resp.Bookings))
	}
}

func TestDeleteBooking(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "testuser", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=booking-1", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d; body = %s", w.Code, http.StatusOK, w.Body.String())
	}
}

func TestDeleteBookingNotFound(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=booking-999", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestDeleteBookingMissingID(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/bookings", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteBookingForbiddenOtherUser(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "alice", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=booking-1", nil)
	req = reqWithUser(req, &UserInfo{Username: "bob", IsAdmin: false})
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestDeleteBookingAdminCanDeleteOthers(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "alice", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=booking-1", nil)
	req = reqWithUser(req, testAdmin())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestDeleteBookingConsumedForbidden(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "testuser", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=booking-1", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want %d", w.Code, http.StatusForbidden)
	}
}

func TestBulkCancelHandler(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	for i := 0; i < 3; i++ {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			"booking-"+string(rune('1'+i)), "testuser", "", "nvidia.com/gpu", i, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	body := `{"ids":["booking-1","booking-2","booking-3"]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Deleted []string `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Deleted) != 3 {
		t.Errorf("deleted count = %d, want 3", len(resp.Deleted))
	}
}

func TestBulkCancelHandlerEmptyIDs(t *testing.T) {
	setupTestDB(t)

	body := `{"ids":[]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkCancelHandlerInvalidID(t *testing.T) {
	setupTestDB(t)

	body := `{"ids":["bad; DROP TABLE bookings"]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkBookingHandler(t *testing.T) {
	setupTestDB(t)

	start := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	end := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	body := `{"resources":{"nvidia.com/gpu":2},"startDate":"` + start + `","endDate":"` + end + `","description":"bulk test","startHour":0,"endHour":24}`

	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", w.Code, http.StatusCreated, w.Body.String())
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
		Errors   []string           `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	// 2 GPUs * 2 days = 4 bookings
	if len(resp.Bookings) != 4 {
		t.Errorf("bookings count = %d, want 4", len(resp.Bookings))
	}
}

func TestBulkBookingHandlerMissingFields(t *testing.T) {
	setupTestDB(t)

	body := `{"resources":{},"startDate":"","endDate":""}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkBookingHandlerInvalidDateRange(t *testing.T) {
	setupTestDB(t)

	body := `{"resources":{"nvidia.com/gpu":1},"startDate":"2025-04-30","endDate":"2025-04-28"}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestDeleteBookingInvalidID(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/bookings?id=invalid;DROP+TABLE", nil)
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	DeleteBooking(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkCancelHandlerMixedOwnership(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	// testuser owns booking-1, alice owns booking-2
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "testuser", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-2", "alice", "", "nvidia.com/gpu", 1, today, "full", "now", database.SourceReserved, "", 0, 24, 0,
	)

	body := `{"ids":["booking-1","booking-2"]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Deleted []string `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Deleted) != 1 {
		t.Errorf("deleted = %d, want 1 (own booking only)", len(resp.Deleted))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("errors = %d, want 1 (forbidden for alice's booking)", len(resp.Errors))
	}
}

func TestBulkCancelHandlerConsumedBooking(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	today := time.Now().Format("2006-01-02")
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"booking-1", "testuser", "", "nvidia.com/gpu", 0, today, "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)

	body := `{"ids":["booking-1"]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Deleted []string `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Deleted) != 0 {
		t.Errorf("deleted = %d, want 0", len(resp.Deleted))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("errors = %d, want 1 (consumed_booking)", len(resp.Errors))
	}
}

func TestBulkCancelHandlerNotFound(t *testing.T) {
	setupTestDB(t)

	body := `{"ids":["booking-999"]}`
	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Deleted []string `json:"deleted"`
		Errors  []string `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Errors) != 1 {
		t.Errorf("errors = %d, want 1 (not_found)", len(resp.Errors))
	}
}

func TestBulkCancelHandlerInvalidJSON(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodDelete, "/bookings/bulk/cancel", strings.NewReader("{bad"))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkCancelHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkBookingHandlerEvictsConsumed(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	// Fill slot 0 with a consumed booking
	db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"consumed-1", "kueue-user", "", "nvidia.com/gpu", 0, date, "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)

	body := `{"resources":{"nvidia.com/gpu":1},"startDate":"` + date + `","endDate":"` + date + `","description":"","startHour":0,"endHour":24}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	// Consumed booking should be evicted
	var count int
	db.QueryRow("SELECT COUNT(*) FROM bookings WHERE id = 'consumed-1'").Scan(&count)
	if count != 0 {
		t.Error("consumed booking should have been evicted")
	}
}

func TestBulkBookingHandlerAllSlotsTaken(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	// Fill all 8 GPU slots with reserved bookings
	for i := range 8 {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			fmt.Sprintf("existing-%d", i), "alice", "", "nvidia.com/gpu", i, date, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	body := `{"resources":{"nvidia.com/gpu":1},"startDate":"` + date + `","endDate":"` + date + `","description":"","startHour":0,"endHour":24}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestBulkBookingHandlerPartialSlots(t *testing.T) {
	setupTestDB(t)
	db := database.DB()

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	// Fill 7 of 8 slots
	for i := range 7 {
		db.Exec(
			"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
			fmt.Sprintf("existing-%d", i), "alice", "", "nvidia.com/gpu", i, date, "full", "now", database.SourceReserved, "", 0, 24, 0,
		)
	}

	// Request 3, only 1 available
	body := `{"resources":{"nvidia.com/gpu":3},"startDate":"` + date + `","endDate":"` + date + `","description":"","startHour":0,"endHour":24}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
		Errors   []string           `json:"errors"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 1 {
		t.Errorf("bookings = %d, want 1 (only 1 slot available)", len(resp.Bookings))
	}
	if len(resp.Errors) != 1 {
		t.Errorf("errors = %d, want 1 (partial availability)", len(resp.Errors))
	}
}

func TestBulkBookingHandlerInvalidJSON(t *testing.T) {
	setupTestDB(t)

	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader("{bad"))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestBulkBookingHandlerHourClamping(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resources":{"nvidia.com/gpu":1},"startDate":"` + date + `","endDate":"` + date + `","description":"","startHour":-10,"endHour":50}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings) != 1 {
		t.Fatalf("bookings = %d, want 1", len(resp.Bookings))
	}
	if resp.Bookings[0].StartHour != 0 {
		t.Errorf("StartHour = %d, want 0", resp.Bookings[0].StartHour)
	}
	if resp.Bookings[0].EndHour != 24 {
		t.Errorf("EndHour = %d, want 24", resp.Bookings[0].EndHour)
	}
}

func TestBulkBookingHandlerDescriptionTruncation(t *testing.T) {
	setupTestDB(t)

	date := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	longDesc := strings.Repeat("d", 200)
	body := `{"resources":{"nvidia.com/gpu":1},"startDate":"` + date + `","endDate":"` + date + `","description":"` + longDesc + `","startHour":0,"endHour":24}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d", w.Code)
	}

	var resp struct {
		Bookings []database.Booking `json:"bookings"`
	}
	json.NewDecoder(w.Body).Decode(&resp)
	if len(resp.Bookings[0].Description) != 160 {
		t.Errorf("description length = %d, want 160", len(resp.Bookings[0].Description))
	}
}

func TestBulkBookingHandlerInvalidResource(t *testing.T) {
	setupTestDB(t)

	start := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	body := `{"resources":{"fake/gpu":1},"startDate":"` + start + `","endDate":"` + start + `"}`
	req := httptest.NewRequest(http.MethodPost, "/bookings/bulk", strings.NewReader(body))
	req = reqWithUser(req, testUser())
	w := httptest.NewRecorder()

	BulkBookingHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
}
