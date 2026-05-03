package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
	"github.com/eformat/gpu-booking-plugin/pkg/kube"
)

func GetBookings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	db := database.DB()

	startDate := time.Now().AddDate(0, 0, -7).Format("2006-01-02")
	endDate := time.Now().AddDate(0, 0, BookingWindowDays+1).Format("2006-01-02")
	rows, err := db.QueryContext(ctx,
		"SELECT "+database.BookingColumns+" FROM bookings WHERE date >= ? AND date < ? ORDER BY date, slot_type",
		startDate, endDate)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to query bookings", "error", err)
		return
	}
	defer rows.Close()

	bookings := []database.Booking{}
	for rows.Next() {
		b, err := database.ScanBooking(rows)
		if err != nil {
			slog.Error("failed to scan booking", "error", err)
			continue
		}
		bookings = append(bookings, b)
	}
	if err := rows.Err(); err != nil {
		slog.Error("failed iterating bookings", "error", err)
	}

	// Build active reservations map: user -> clusterqueue name
	activeRes := map[string]string{}
	now := time.Now().UTC()
	for _, b := range bookings {
		if b.Source != database.SourceReserved {
			continue
		}
		base, err := time.Parse("2006-01-02", b.Date)
		if err != nil {
			continue
		}
		utcStart := base.Add(time.Duration(b.StartHour-b.UtcOffset) * time.Hour)
		utcEnd := base.Add(time.Duration(b.EndHour-b.UtcOffset) * time.Hour)
		if !now.Before(utcStart) && now.Before(utcEnd) {
			if _, ok := activeRes[b.User]; !ok {
				activeRes[b.User] = "user-" + b.User
			}
		}
	}

	JsonResponse(w, map[string]any{
		"bookings":           bookings,
		"activeReservations": activeRes,
		"currentUser":        user.Username,
	})
}

func CreateBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	db := database.DB()

	var req struct {
		Resource    string `json:"resource"`
		SlotIndex   int    `json:"slotIndex"`
		Date        string `json:"date"`
		SlotType    string `json:"slotType"`
		Description string `json:"description"`
		StartHour   int    `json:"startHour"`
		EndHour     int    `json:"endHour"`
		UtcOffset   int    `json:"utcOffset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	if req.SlotType != database.SlotTypeFull {
		HttpError(w, http.StatusBadRequest, "invalid_slot_type")
		return
	}

	// Validate resource type exists
	spec, ok := database.GPUSpecByType(req.Resource)
	if !ok {
		HttpError(w, http.StatusBadRequest, "invalid_resource")
		return
	}

	// Validate slot index is within resource bounds
	if req.SlotIndex < 0 || req.SlotIndex >= spec.Count {
		HttpError(w, http.StatusBadRequest, "invalid_slot_index")
		return
	}

	// Validate date format and bounds
	if !IsValidBookingDate(req.Date, BookingWindowDays, req.UtcOffset) {
		HttpError(w, http.StatusBadRequest, "invalid_date")
		return
	}

	// Check for conflicts
	rows, err := db.QueryContext(ctx,
		"SELECT id, source FROM bookings WHERE resource = ? AND slot_index = ? AND date = ?",
		req.Resource, req.SlotIndex, req.Date,
	)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to check booking conflicts", "error", err)
		return
	}
	var conflictIDs []string
	hasReservedConflict := false
	for rows.Next() {
		var cID, cSource string
		if err := rows.Scan(&cID, &cSource); err != nil {
			continue
		}
		if cSource == database.SourceReserved {
			hasReservedConflict = true
		}
		conflictIDs = append(conflictIDs, cID)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Error("failed iterating conflict check", "error", err)
	}

	if hasReservedConflict {
		JsonResponseStatus(w, http.StatusConflict, map[string]string{"error": "slot_taken"})
		return
	}

	// Evict consumed bookings
	for _, cID := range conflictIDs {
		if _, err := db.ExecContext(ctx, "DELETE FROM bookings WHERE id = ?", cID); err != nil {
			slog.Error("failed to evict consumed booking", "bookingId", cID, "error", err)
		} else {
			slog.Info("evicted consumed booking for reservation", "bookingId", cID, "user", user.Username)
		}
	}

	id := fmt.Sprintf("booking-%d", time.Now().UnixNano())
	createdAt := time.Now().UTC().Format(time.RFC3339)

	desc := req.Description
	if len(desc) > 160 {
		desc = desc[:160]
	}

	startHour := req.StartHour
	endHour := req.EndHour
	if startHour < 0 || startHour > 23 {
		startHour = 0
	}
	if endHour < 1 || endHour > 24 {
		endHour = 24
	}

	utcOffset := req.UtcOffset
	if utcOffset < -12 || utcOffset > 14 {
		utcOffset = 0
	}

	_, err = db.ExecContext(ctx,
		"INSERT INTO bookings (id, user, email, resource, slot_index, date, slot_type, created_at, source, description, start_hour, end_hour, utc_offset) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, user.Username, "", req.Resource, req.SlotIndex, req.Date, req.SlotType, createdAt, database.SourceReserved, desc, startHour, endHour, utcOffset,
	)
	if err != nil {
		JsonResponseStatus(w, http.StatusConflict, map[string]string{"error": "slot_taken"})
		return
	}

	booking := database.Booking{
		ID:          id,
		User:        user.Username,
		Resource:    req.Resource,
		SlotIndex:   req.SlotIndex,
		Date:        req.Date,
		SlotType:    req.SlotType,
		CreatedAt:   createdAt,
		Source:      database.SourceReserved,
		Description: desc,
		StartHour:   startHour,
		EndHour:     endHour,
		UtcOffset:   utcOffset,
	}

	slog.Info("AUDIT: booking created", "user", user.Username, "bookingId", id, "resource", req.Resource, "slot", req.SlotIndex, "date", req.Date, "remote_addr", r.RemoteAddr)
	JsonResponseStatus(w, http.StatusCreated, booking)
	kube.TriggerSyncReservations()
}

func DeleteBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.URL.Query().Get("id")
	if id == "" {
		HttpError(w, http.StatusBadRequest, "missing_id")
		return
	}
	if !IsValidBookingID(id) {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	user := GetUser(r)
	db := database.DB()

	var owner, source string
	err := db.QueryRowContext(ctx, "SELECT user, source FROM bookings WHERE id = ?", id).Scan(&owner, &source)
	if err == sql.ErrNoRows {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	if source == database.SourceConsumed {
		JsonResponseStatus(w, http.StatusForbidden, map[string]string{"error": "consumed_booking"})
		return
	}

	if owner != user.Username && !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "forbidden")
		return
	}

	_, err = db.ExecContext(ctx, "DELETE FROM bookings WHERE id = ?", id)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	slog.Info("AUDIT: booking deleted", "user", user.Username, "bookingId", id, "owner", owner, "remote_addr", r.RemoteAddr)
	JsonResponse(w, map[string]string{"status": "deleted"})
	kube.TriggerSyncReservations()
}

func BulkCancelHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	db := database.DB()

	var req struct {
		IDs []string `json:"ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	for _, id := range req.IDs {
		if !IsValidBookingID(id) {
			HttpError(w, http.StatusBadRequest, "invalid_id")
			return
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to start transaction", "error", err)
		return
	}
	defer tx.Rollback()

	var deleted []string
	var errors []string
	for _, id := range req.IDs {
		var owner, source string
		err := tx.QueryRowContext(ctx, "SELECT user, source FROM bookings WHERE id = ?", id).Scan(&owner, &source)
		if err == sql.ErrNoRows {
			errors = append(errors, fmt.Sprintf("%s: not_found", id))
			continue
		}
		if err != nil {
			errors = append(errors, fmt.Sprintf("%s: database_error", id))
			continue
		}
		if source == database.SourceConsumed {
			errors = append(errors, fmt.Sprintf("%s: consumed_booking", id))
			continue
		}
		if owner != user.Username && !user.IsAdmin {
			errors = append(errors, fmt.Sprintf("%s: forbidden", id))
			continue
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM bookings WHERE id = ?", id); err != nil {
			errors = append(errors, fmt.Sprintf("%s: database_error", id))
			continue
		}
		deleted = append(deleted, id)
	}

	if err := tx.Commit(); err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to commit bulk cancel", "error", err)
		return
	}

	if len(deleted) > 0 {
		slog.Info("AUDIT: bulk cancel", "user", user.Username, "deleted_count", len(deleted), "error_count", len(errors), "remote_addr", r.RemoteAddr)
	}

	JsonResponse(w, map[string]any{
		"deleted": deleted,
		"errors":  errors,
	})

	if len(deleted) > 0 {
		kube.TriggerSyncReservations()
	}
}

func BulkBookingHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	db := database.DB()

	var req struct {
		Resources   map[string]int `json:"resources"`
		StartDate   string         `json:"startDate"`
		EndDate     string         `json:"endDate"`
		Description string         `json:"description"`
		StartHour   int            `json:"startHour"`
		EndHour     int            `json:"endHour"`
		UtcOffset   int            `json:"utcOffset"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	if req.StartDate == "" || req.EndDate == "" || len(req.Resources) == 0 {
		HttpError(w, http.StatusBadRequest, "missing_fields")
		return
	}

	if req.StartDate > req.EndDate {
		HttpError(w, http.StatusBadRequest, "invalid_date_range")
		return
	}

	// Validate all resource types exist
	for resource := range req.Resources {
		if _, ok := database.GPUSpecByType(resource); !ok {
			HttpError(w, http.StatusBadRequest, fmt.Sprintf("invalid_resource: %s", resource))
			return
		}
	}

	desc := req.Description
	if len(desc) > 160 {
		desc = desc[:160]
	}

	startHour := req.StartHour
	endHour := req.EndHour
	if startHour < 0 || startHour > 23 {
		startHour = 0
	}
	if endHour < 1 || endHour > 24 {
		endHour = 24
	}

	utcOffset := req.UtcOffset
	if utcOffset < -12 || utcOffset > 14 {
		utcOffset = 0
	}

	start, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_start_date")
		return
	}
	end, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_end_date")
		return
	}

	var dates []string
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	cfg := database.GetConfig(BookingWindowDays)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to start transaction", "error", err)
		return
	}
	defer tx.Rollback()

	var created []database.Booking
	var errors []string

	// Collect requested resources (skip zero counts)
	var resources []string
	for resource, count := range req.Resources {
		if count > 0 {
			resources = append(resources, resource)
		}
	}

	// Batch fetch all existing bookings for requested resources and dates in one query
	type slotKey struct {
		resource string
		date     string
	}
	type slotInfo struct {
		reserved    map[int]bool
		consumedIDs map[int]string
	}
	slotMap := map[slotKey]*slotInfo{}

	if len(resources) > 0 && len(dates) > 0 {
		args := make([]any, 0, len(resources)+len(dates))
		resPlaceholders := make([]byte, 0, len(resources)*2)
		for i, r := range resources {
			if i > 0 {
				resPlaceholders = append(resPlaceholders, ',')
			}
			resPlaceholders = append(resPlaceholders, '?')
			args = append(args, r)
		}
		datePlaceholders := make([]byte, 0, len(dates)*2)
		for i, d := range dates {
			if i > 0 {
				datePlaceholders = append(datePlaceholders, ',')
			}
			datePlaceholders = append(datePlaceholders, '?')
			args = append(args, d)
		}

		query := fmt.Sprintf(
			"SELECT resource, date, slot_index, source, id FROM bookings WHERE resource IN (%s) AND date IN (%s)",
			string(resPlaceholders), string(datePlaceholders),
		)
		slotRows, err := tx.QueryContext(ctx, query, args...)
		if err != nil {
			HttpError(w, http.StatusInternalServerError, "database_error")
			slog.Error("failed to batch query bookings", "error", err)
			return
		}
		for slotRows.Next() {
			var res, date, src, bid string
			var idx int
			if err := slotRows.Scan(&res, &date, &idx, &src, &bid); err != nil {
				slog.Error("failed scanning slot row", "error", err)
				continue
			}
			key := slotKey{resource: res, date: date}
			info, ok := slotMap[key]
			if !ok {
				info = &slotInfo{reserved: map[int]bool{}, consumedIDs: map[int]string{}}
				slotMap[key] = info
			}
			if src == database.SourceReserved {
				info.reserved[idx] = true
			} else {
				info.consumedIDs[idx] = bid
			}
		}
		slotRows.Close()
		if err := slotRows.Err(); err != nil {
			slog.Error("failed iterating batch slot query", "error", err)
		}
	}

	// Process each resource/date using pre-fetched data
	for _, resource := range resources {
		count := req.Resources[resource]
		for _, date := range dates {
			key := slotKey{resource: resource, date: date}
			info := slotMap[key]
			if info == nil {
				info = &slotInfo{reserved: map[int]bool{}, consumedIDs: map[int]string{}}
			}

			maxUnits := 0
			for _, gr := range cfg.Resources {
				if gr.Type == resource {
					maxUnits = gr.Count
					break
				}
			}
			if maxUnits == 0 {
				errors = append(errors, fmt.Sprintf("%s: unknown resource type", resource))
				continue
			}

			booked := 0
			for unitIdx := 0; unitIdx < maxUnits && booked < count; unitIdx++ {
				if info.reserved[unitIdx] {
					continue
				}
				if cID, ok := info.consumedIDs[unitIdx]; ok {
					if _, err := tx.ExecContext(ctx, "DELETE FROM bookings WHERE id = ?", cID); err != nil {
						slog.Error("bulk booking: failed to evict consumed booking", "bookingId", cID, "error", err)
						continue
					}
					slog.Info("bulk booking: evicted consumed booking", "bookingId", cID, "user", user.Username)
				}

				id := fmt.Sprintf("booking-%d", time.Now().UnixNano())
				createdAt := time.Now().UTC().Format(time.RFC3339)

				_, err := tx.ExecContext(ctx,
					"INSERT INTO bookings (id, user, email, resource, slot_index, date, slot_type, created_at, source, description, start_hour, end_hour, utc_offset) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
					id, user.Username, "", resource, unitIdx, date, database.SlotTypeFull, createdAt, database.SourceReserved, desc, startHour, endHour, utcOffset,
				)
				if err != nil {
					slog.Error("bulk booking: insert failed", "resource", resource, "slot", unitIdx, "date", date, "error", err)
					continue
				}

				created = append(created, database.Booking{
					ID:          id,
					User:        user.Username,
					Resource:    resource,
					SlotIndex:   unitIdx,
					Date:        date,
					SlotType:    database.SlotTypeFull,
					CreatedAt:   createdAt,
					Source:      database.SourceReserved,
					Description: desc,
					StartHour:   startHour,
					EndHour:     endHour,
					UtcOffset:   utcOffset,
				})
				booked++
			}
			if booked < count {
				errors = append(errors, fmt.Sprintf("%s on %s: only %d of %d slots available", resource, date, booked, count))
			}
		}
	}

	if len(created) == 0 && len(errors) > 0 {
		// Rollback is handled by defer
		JsonResponseStatus(w, http.StatusConflict, map[string]any{"error": "no_slots_available", "details": errors})
		return
	}

	if err := tx.Commit(); err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		slog.Error("failed to commit bulk booking", "error", err)
		return
	}

	slog.Info("AUDIT: bulk booking created", "user", user.Username, "created_count", len(created), "error_count", len(errors), "start_date", req.StartDate, "end_date", req.EndDate, "remote_addr", r.RemoteAddr)

	JsonResponseStatus(w, http.StatusCreated, map[string]any{
		"bookings": created,
		"errors":   errors,
	})

	if len(created) > 0 {
		kube.TriggerSyncReservations()
	}
}
