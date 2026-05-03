package api

import (
	"testing"
	"time"
)

func TestIsValidBookingID(t *testing.T) {
	valid := []string{
		"booking-1",
		"booking-123456",
		"kueue-my-ns-gpu-s0-2025-04-24",
		"kueue-team-alpha-mig-3g-s1-2025-12-31",
	}
	for _, id := range valid {
		if !IsValidBookingID(id) {
			t.Errorf("expected valid: %q", id)
		}
	}

	invalid := []string{
		"",
		"booking-",
		"booking-abc",
		"random-string",
		"BOOKING-123",
		"kueue-",
		"kueue-ns-res-0-2025-01-01",        // missing s prefix on slot
		"kueue-NS-res-s0-2025-01-01",        // uppercase namespace
		"booking-1; DROP TABLE bookings; --", // injection attempt
	}
	for _, id := range invalid {
		if IsValidBookingID(id) {
			t.Errorf("expected invalid: %q", id)
		}
	}

	long := "booking-" + string(make([]byte, 300))
	if IsValidBookingID(long) {
		t.Error("expected rejection for ID exceeding 256 chars")
	}
}

func TestIsValidBookingDate(t *testing.T) {
	today := time.Now().Truncate(24 * time.Hour)
	window := 30

	todayStr := today.Format("2006-01-02")
	if !IsValidBookingDate(todayStr, window, 0) {
		t.Errorf("today should be valid: %s", todayStr)
	}

	future := today.AddDate(0, 0, 15).Format("2006-01-02")
	if !IsValidBookingDate(future, window, 0) {
		t.Errorf("15 days from now should be valid: %s", future)
	}

	yesterday := today.AddDate(0, 0, -1).Format("2006-01-02")
	if IsValidBookingDate(yesterday, window, 0) {
		t.Errorf("yesterday should be invalid: %s", yesterday)
	}

	tooFar := today.AddDate(0, 0, window+5).Format("2006-01-02")
	if IsValidBookingDate(tooFar, window, 0) {
		t.Errorf("beyond window should be invalid: %s", tooFar)
	}

	if IsValidBookingDate("not-a-date", window, 0) {
		t.Error("garbage string should be invalid")
	}

	if IsValidBookingDate("", window, 0) {
		t.Error("empty string should be invalid")
	}

	boundaryDate := today.AddDate(0, 0, window).Format("2006-01-02")
	if !IsValidBookingDate(boundaryDate, window, 0) {
		t.Errorf("last day of window should be valid: %s", boundaryDate)
	}

	pastBoundary := today.AddDate(0, 0, window+1).Format("2006-01-02")
	if IsValidBookingDate(pastBoundary, window, 0) {
		t.Errorf("one day past window should be invalid: %s", pastBoundary)
	}
}

func TestIsValidBookingDateWithOffset(t *testing.T) {
	// AEST user (UTC+10): their "today" is ahead of UTC.
	// At 22:00 UTC on May 3, it's already May 4 in AEST.
	// A booking for "May 4" should be valid for a UTC+10 user even
	// when UTC is still May 3.
	utcNow := time.Now().UTC()
	userNow := utcNow.Add(10 * time.Hour)
	userToday := time.Date(userNow.Year(), userNow.Month(), userNow.Day(), 0, 0, 0, 0, time.UTC)
	userTodayStr := userToday.Format("2006-01-02")

	if !IsValidBookingDate(userTodayStr, 30, 10) {
		t.Errorf("AEST user's today should be valid: %s", userTodayStr)
	}

	// EST user (UTC-5): their "today" is behind UTC.
	userNowEST := utcNow.Add(-5 * time.Hour)
	userTodayEST := time.Date(userNowEST.Year(), userNowEST.Month(), userNowEST.Day(), 0, 0, 0, 0, time.UTC)
	userTodayESTStr := userTodayEST.Format("2006-01-02")

	if !IsValidBookingDate(userTodayESTStr, 30, -5) {
		t.Errorf("EST user's today should be valid: %s", userTodayESTStr)
	}
}
