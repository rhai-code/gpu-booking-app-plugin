package kube

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func setupTestDB(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := database.Init(filepath.Join(dir, "test.db")); err != nil {
		t.Fatalf("Init test DB: %v", err)
	}
	t.Cleanup(database.Close)
}

func insertBooking(t *testing.T, id, user, resource string, slotIndex int, date string, startHour, endHour, utcOffset int) {
	t.Helper()
	db := database.DB()
	_, err := db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		id, user, "", resource, slotIndex, date, "full", "now", database.SourceReserved, "", startHour, endHour, utcOffset,
	)
	if err != nil {
		t.Fatalf("insertBooking %s: %v", id, err)
	}
}

func TestActiveReservations_UTC_FullDay(t *testing.T) {
	setupTestDB(t)

	// UTC user books a full day for "2026-05-04" with offset=0.
	// Active window: May 4 00:00 UTC to May 5 00:00 UTC.
	insertBooking(t, "b1", "alice", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	// At May 4 12:00 UTC — should be active.
	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(res))
	}
	if res[0].User != "alice" {
		t.Errorf("user = %q, want alice", res[0].User)
	}
	if res[0].Resources["nvidia.com/gpu"] != 1 {
		t.Errorf("gpu count = %d, want 1", res[0].Resources["nvidia.com/gpu"])
	}
}

func TestActiveReservations_UTC_FullDay_Expired(t *testing.T) {
	setupTestDB(t)

	// UTC user books May 4 full day. At May 5 01:00 it's expired.
	insertBooking(t, "b1", "alice", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)

	now := time.Date(2026, 5, 5, 1, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 reservations (expired), got %d", len(res))
	}
}

func TestActiveReservations_AEST_FullDay(t *testing.T) {
	setupTestDB(t)

	// AEST user (UTC+10) books "2026-05-04" full day (local hours 0-24).
	// UTC active window: May 3 14:00 UTC to May 4 14:00 UTC.
	insertBooking(t, "b1", "bob", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 10)

	// At May 3 22:00 UTC (= May 4 08:00 AEST) — should be active.
	now := time.Date(2026, 5, 3, 22, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("AEST booking should be active at May 3 22:00 UTC, got %d reservations", len(res))
	}
	if res[0].User != "bob" {
		t.Errorf("user = %q, want bob", res[0].User)
	}
}

func TestActiveReservations_AEST_NotYetActive(t *testing.T) {
	setupTestDB(t)

	// AEST user (UTC+10) books "2026-05-04" full day.
	// UTC start is May 3 14:00. At May 3 13:00 it shouldn't be active yet.
	insertBooking(t, "b1", "bob", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 10)

	now := time.Date(2026, 5, 3, 13, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("AEST booking should NOT be active at May 3 13:00 UTC, got %d", len(res))
	}
}

func TestActiveReservations_EST_FullDay(t *testing.T) {
	setupTestDB(t)

	// EST user (UTC-5) books "2026-05-04" full day (local hours 0-24).
	// UTC active window: May 4 05:00 UTC to May 5 05:00 UTC.
	insertBooking(t, "b1", "carol", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, -5)

	// At May 4 10:00 UTC (= May 4 05:00 EST) — should be active.
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("EST booking should be active at May 4 10:00 UTC, got %d", len(res))
	}

	// At May 4 04:00 UTC (= May 3 23:00 EST) — should NOT be active yet.
	now = time.Date(2026, 5, 4, 4, 0, 0, 0, time.UTC)
	res, err = getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("EST booking should NOT be active at May 4 04:00 UTC, got %d", len(res))
	}
}

func TestActiveReservations_PartialDay_CrossMidnight(t *testing.T) {
	setupTestDB(t)

	// AEST user (UTC+10) books 9am-5pm local on May 4.
	// UTC: start = (9 - 10) = -1h from May 4 midnight UTC = May 3 23:00 UTC
	//      end   = (17 - 10) = 7h from May 4 midnight UTC = May 4 07:00 UTC
	insertBooking(t, "b1", "dave", "nvidia.com/gpu", 0, "2026-05-04", 9, 17, 10)

	// At May 4 03:00 UTC (= May 4 13:00 AEST) — should be active.
	now := time.Date(2026, 5, 4, 3, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("partial-day AEST booking should be active at May 4 03:00 UTC, got %d", len(res))
	}

	// At May 3 22:00 UTC (= before 9am AEST, which is 23:00 UTC) — NOT active.
	now = time.Date(2026, 5, 3, 22, 0, 0, 0, time.UTC)
	res, err = getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("partial-day should NOT be active at May 3 22:00 UTC, got %d", len(res))
	}

	// At May 4 07:30 UTC (= after 5pm AEST which is 07:00 UTC) — NOT active.
	now = time.Date(2026, 5, 4, 7, 30, 0, 0, time.UTC)
	res, err = getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("partial-day should NOT be active at May 4 07:30 UTC, got %d", len(res))
	}
}

func TestActiveReservations_MultipleUsers_DifferentTZ(t *testing.T) {
	setupTestDB(t)

	// AEST user full day May 4: UTC window May 3 14:00 – May 4 14:00
	insertBooking(t, "b1", "aest-user", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 10)
	// EST user full day May 4: UTC window May 4 05:00 – May 5 05:00
	insertBooking(t, "b2", "est-user", "nvidia.com/gpu", 1, "2026-05-04", 0, 24, -5)

	// At May 4 10:00 UTC — both should be active (AEST window: 14:00-14:00, EST: 05:00-05:00)
	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("expected 2 reservations (both active), got %d", len(res))
	}

	// At May 4 15:00 UTC — only EST should be active (AEST expired at 14:00)
	now = time.Date(2026, 5, 4, 15, 0, 0, 0, time.UTC)
	res, err = getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation (only EST), got %d", len(res))
	}
	if res[0].User != "est-user" {
		t.Errorf("expected est-user, got %q", res[0].User)
	}
}

func TestActiveReservations_MultipleSlots_SameUser(t *testing.T) {
	setupTestDB(t)

	// Same user books 2 GPU slots for the same day.
	insertBooking(t, "b1", "alice", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 0)
	insertBooking(t, "b2", "alice", "nvidia.com/gpu", 1, "2026-05-04", 0, 24, 0)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation (one user), got %d", len(res))
	}
	if res[0].Resources["nvidia.com/gpu"] != 2 {
		t.Errorf("gpu count = %d, want 2", res[0].Resources["nvidia.com/gpu"])
	}
}

func TestActiveReservations_UntilTimestamp(t *testing.T) {
	setupTestDB(t)

	// AEST user full day May 4, offset +10.
	// UTC end = May 4 00:00 + (24-10)h = May 4 14:00 UTC
	insertBooking(t, "b1", "bob", "nvidia.com/gpu", 0, "2026-05-04", 0, 24, 10)

	now := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("expected 1 reservation, got %d", len(res))
	}

	expectedUntil := time.Date(2026, 5, 4, 14, 0, 0, 0, time.UTC).Unix()
	if res[0].Until != expectedUntil {
		t.Errorf("until = %d, want %d (May 4 14:00 UTC)", res[0].Until, expectedUntil)
	}
}

func TestActiveReservations_ConsumedBookingsIgnored(t *testing.T) {
	setupTestDB(t)

	db := database.DB()
	// Insert a consumed booking (should be ignored by getActiveReservationsAt)
	_, err := db.Exec(
		"INSERT INTO bookings ("+database.BookingColumns+") VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)",
		"c1", "kueue-user", "", "nvidia.com/gpu", 0, "2026-05-04", "full", "now", database.SourceConsumed, "", 0, 24, 0,
	)
	if err != nil {
		t.Fatalf("insert consumed: %v", err)
	}

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("consumed bookings should be ignored, got %d reservations", len(res))
	}
}

func TestActiveReservations_Empty(t *testing.T) {
	setupTestDB(t)

	now := time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC)
	res, err := getActiveReservationsAt(now)
	if err != nil {
		t.Fatalf("getActiveReservationsAt: %v", err)
	}
	if len(res) != 0 {
		t.Errorf("expected 0 reservations, got %d", len(res))
	}
}
