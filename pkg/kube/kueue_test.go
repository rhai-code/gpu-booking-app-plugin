package kube

import (
	"testing"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
)

func countBookings(t *testing.T, resource, date, source string) int {
	t.Helper()
	var count int
	err := database.DB().QueryRow(
		"SELECT COUNT(*) FROM bookings WHERE resource = ? AND date = ? AND source = ?",
		resource, date, source,
	).Scan(&count)
	if err != nil {
		t.Fatalf("countBookings: %v", err)
	}
	return count
}

func getBookingSlots(t *testing.T, resource, date, source string) []int {
	t.Helper()
	rows, err := database.DB().Query(
		"SELECT slot_index FROM bookings WHERE resource = ? AND date = ? AND source = ? ORDER BY slot_index",
		resource, date, source,
	)
	if err != nil {
		t.Fatalf("getBookingSlots: %v", err)
	}
	defer rows.Close()
	var slots []int
	for rows.Next() {
		var idx int
		if rows.Scan(&idx) == nil {
			slots = append(slots, idx)
		}
	}
	return slots
}

func TestSyncBookings_NoReservations(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 2},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	got := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if got != 2 {
		t.Errorf("consumed count = %d, want 2", got)
	}

	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if len(slots) != 2 || slots[0] != 0 || slots[1] != 1 {
		t.Errorf("consumed slots = %v, want [0 1]", slots)
	}
}

func TestSyncBookings_ReservationFulfilled(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// alice reserves slot 0
	insertBooking(t, "res-1", "alice", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)

	// alice has 1 running workload from Kueue
	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	// Reservation should still be there
	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 1 {
		t.Errorf("reserved count = %d, want 1", reserved)
	}

	// No consumed booking should be created (reservation already covers it)
	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 0 {
		t.Errorf("consumed count = %d, want 0", consumed)
	}
}

func TestSyncBookings_ReservationPlusExtraWorkload(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// alice reserves slot 0
	insertBooking(t, "res-1", "alice", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)

	// alice has 2 running workloads (1 covered by reservation, 1 extra)
	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 2},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 1 {
		t.Errorf("reserved count = %d, want 1", reserved)
	}

	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 1 {
		t.Errorf("consumed count = %d, want 1", consumed)
	}

	// The consumed booking should be on slot 1 (slot 0 is reserved)
	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if len(slots) != 1 || slots[0] != 1 {
		t.Errorf("consumed slots = %v, want [1]", slots)
	}
}

func TestSyncBookings_DifferentUserReservation(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// bob reserves slot 0
	insertBooking(t, "res-1", "bob", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)

	// alice has 1 running workload
	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	// bob's reservation untouched
	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 1 {
		t.Errorf("reserved count = %d, want 1", reserved)
	}

	// alice's consumed booking should be on slot 1 (slot 0 taken by bob)
	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 1 {
		t.Errorf("consumed count = %d, want 1", consumed)
	}

	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if len(slots) != 1 || slots[0] != 1 {
		t.Errorf("consumed slots = %v, want [1]", slots)
	}
}

func TestSyncBookings_MultipleUsersNoOverlap(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
		{Namespace: "user-bob", User: "bob", Resource: "nvidia.com/gpu", Count: 1},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 2 {
		t.Errorf("consumed count = %d, want 2", consumed)
	}

	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if len(slots) != 2 || slots[0] != 0 || slots[1] != 1 {
		t.Errorf("consumed slots = %v, want [0 1]", slots)
	}
}

func TestSyncBookings_DeterministicSlotOrder(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// Run twice with same usages — slots should be identical
	usages := []resourceUsage{
		{Namespace: "user-charlie", User: "charlie", Resource: "nvidia.com/gpu", Count: 1},
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
	}

	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings first: %v", err)
	}
	slots1 := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)

	// Clear consumed and re-sync
	database.DB().Exec("DELETE FROM bookings WHERE source = ?", database.SourceConsumed)

	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings second: %v", err)
	}
	slots2 := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)

	if len(slots1) != len(slots2) {
		t.Fatalf("slot counts differ: %v vs %v", slots1, slots2)
	}
	for i := range slots1 {
		if slots1[i] != slots2[i] {
			t.Errorf("slot[%d] = %d then %d, want deterministic", i, slots1[i], slots2[i])
		}
	}
}

func TestSyncBookings_Idempotent(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
	}

	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings first: %v", err)
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings second: %v", err)
	}

	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 1 {
		t.Errorf("consumed count = %d after double sync, want 1", consumed)
	}
}

func TestSyncBookings_StaleBookingsRemoved(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// First sync: alice has a workload
	usages := []resourceUsage{
		{Namespace: "user-alice", User: "alice", Resource: "nvidia.com/gpu", Count: 1},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings first: %v", err)
	}
	if c := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed); c != 1 {
		t.Fatalf("consumed after first sync = %d, want 1", c)
	}

	// Second sync: alice's workload is gone
	if err := syncBookings([]resourceUsage{}, dates); err != nil {
		t.Fatalf("syncBookings second: %v", err)
	}

	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 0 {
		t.Errorf("consumed count = %d after workload removed, want 0", consumed)
	}
}

func TestSyncBookings_EmailUserMatchesReservation(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// ltsai@redhat.com reserves slot 0 (via OpenShift UI)
	insertBooking(t, "res-1", "ltsai@redhat.com", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)

	// Kueue sync reports user as "ltsai" (from namespace label)
	usages := []resourceUsage{
		{Namespace: "user-ltsai", User: "ltsai", Resource: "nvidia.com/gpu", Count: 1},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 1 {
		t.Errorf("reserved count = %d, want 1", reserved)
	}

	// Consumed booking should NOT be created — reservation covers it
	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 0 {
		t.Errorf("consumed count = %d, want 0 (reservation should cover it)", consumed)
	}
}

func TestSyncBookings_EmailUserPartialCoverage(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// ltsai@redhat.com reserves 2 slots
	insertBooking(t, "res-1", "ltsai@redhat.com", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)
	insertBooking(t, "res-2", "ltsai@redhat.com", "nvidia.com/gpu", 1, "2026-06-01", 0, 24, 0)

	// Kueue reports 4 consumed units as "ltsai"
	usages := []resourceUsage{
		{Namespace: "user-ltsai", User: "ltsai", Resource: "nvidia.com/gpu", Count: 4},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 2 {
		t.Errorf("reserved count = %d, want 2", reserved)
	}

	// Only 2 consumed bookings should be created (4 total - 2 covered by reservations)
	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 2 {
		t.Errorf("consumed count = %d, want 2", consumed)
	}

	// Consumed slots should not overlap with reserved slots 0,1
	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	for _, s := range slots {
		if s == 0 || s == 1 {
			t.Errorf("consumed slot %d overlaps with reserved slot", s)
		}
	}
}

func TestSyncBookings_MultiNamespaceSameUser(t *testing.T) {
	setupTestDB(t)
	dates := []string{"2026-06-01"}

	// alice@example.com reserves 2 slots (via OpenShift UI)
	insertBooking(t, "res-1", "alice@example.com", "nvidia.com/gpu", 0, "2026-06-01", 0, 24, 0)
	insertBooking(t, "res-2", "alice@example.com", "nvidia.com/gpu", 1, "2026-06-01", 0, 24, 0)

	// Kueue reports 2 consumed in ns-a + 4 consumed in ns-b, both owned by "alice"
	usages := []resourceUsage{
		{Namespace: "ns-a", User: "alice", Resource: "nvidia.com/gpu", Count: 2},
		{Namespace: "ns-b", User: "alice", Resource: "nvidia.com/gpu", Count: 4},
	}
	if err := syncBookings(usages, dates); err != nil {
		t.Fatalf("syncBookings: %v", err)
	}

	reserved := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceReserved)
	if reserved != 2 {
		t.Errorf("reserved count = %d, want 2", reserved)
	}

	// 6 total consumed - 2 covered by reservations = 4 consumed bookings
	consumed := countBookings(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	if consumed != 4 {
		t.Errorf("consumed count = %d, want 4", consumed)
	}

	// Consumed slots must not overlap with reserved slots 0 and 1
	slots := getBookingSlots(t, "nvidia.com/gpu", "2026-06-01", database.SourceConsumed)
	for _, s := range slots {
		if s == 0 || s == 1 {
			t.Errorf("consumed slot %d overlaps with reserved slot", s)
		}
	}
}

func TestNormalizeUser(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"ltsai@redhat.com", "ltsai"},
		{"alice", "alice"},
		{"bob@example.org", "bob"},
		{"", ""},
	}
	for _, tc := range tests {
		got := normalizeUser(tc.input)
		if got != tc.want {
			t.Errorf("normalizeUser(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}
