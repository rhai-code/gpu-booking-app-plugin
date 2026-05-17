package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/eformat/gpu-booking-plugin/pkg/database"
	"github.com/eformat/gpu-booking-plugin/pkg/kube"
)

func AdminListBookings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	db := database.DB()

	// Pagination: limit (default 100, max 1000) and offset (default 0)
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	// Server-side filters
	where := "WHERE 1=1"
	args := []any{}

	if source := r.URL.Query().Get("source"); source != "" {
		where += " AND source = ?"
		args = append(args, source)
	}
	if resource := r.URL.Query().Get("resource"); resource != "" {
		where += " AND resource = ?"
		args = append(args, resource)
	}
	if search := r.URL.Query().Get("search"); search != "" {
		where += " AND (user LIKE ? OR date LIKE ? OR resource LIKE ? OR source LIKE ? OR description LIKE ?)"
		q := "%" + search + "%"
		args = append(args, q, q, q, q, q)
	}

	// Get total count (with filters)
	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM bookings "+where, countArgs...).Scan(&total); err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	queryArgs := append(args, limit, offset)
	rows, err := db.QueryContext(ctx, "SELECT "+database.BookingColumns+" FROM bookings "+where+" ORDER BY date, slot_type LIMIT ? OFFSET ?", queryArgs...)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
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
		slog.Error("failed iterating admin bookings", "error", err)
	}

	JsonResponse(w, map[string]any{
		"bookings":               bookings,
		"total":                  total,
		"limit":                  limit,
		"offset":                 offset,
		"config":                 database.GetConfig(BookingWindowDays),
		"totalSlots":             40,
		"reservationSyncEnabled": kube.ReservationSyncEnabled,
	})
}

func AdminDeleteBooking(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	db := database.DB()
	id := r.URL.Query().Get("id")
	before := r.URL.Query().Get("before")

	// Delete bookings before a given date
	if before != "" {
		if _, err := time.Parse("2006-01-02", before); err != nil {
			HttpError(w, http.StatusBadRequest, "invalid_date")
			return
		}
		result, err := db.ExecContext(ctx, "DELETE FROM bookings WHERE date < ?", before)
		if err != nil {
			HttpError(w, http.StatusInternalServerError, "database_error")
			return
		}
		rows, _ := result.RowsAffected()
		slog.Info("AUDIT: admin delete old bookings", "user", user.Username, "before", before, "deleted_count", rows, "remote_addr", r.RemoteAddr)
		JsonResponse(w, map[string]any{"status": "deleted", "count": rows})
		kube.TriggerSyncReservations()
		return
	}

	// Delete all bookings when no id (admin bulk delete)
	if id == "" {
		result, err := db.ExecContext(ctx, "DELETE FROM bookings")
		if err != nil {
			HttpError(w, http.StatusInternalServerError, "database_error")
			return
		}
		rows, _ := result.RowsAffected()
		slog.Info("AUDIT: admin delete all bookings", "user", user.Username, "deleted_count", rows, "remote_addr", r.RemoteAddr)
		JsonResponse(w, map[string]any{"status": "deleted", "count": rows})
		kube.TriggerSyncReservations()
		return
	}

	if !IsValidBookingID(id) {
		HttpError(w, http.StatusBadRequest, "invalid_id")
		return
	}

	result, err := db.ExecContext(ctx, "DELETE FROM bookings WHERE id = ?", id)
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "database_error")
		return
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		HttpError(w, http.StatusNotFound, "not_found")
		return
	}

	slog.Info("AUDIT: admin delete booking", "user", user.Username, "bookingId", id, "remote_addr", r.RemoteAddr)
	JsonResponse(w, map[string]string{"status": "deleted"})
	kube.TriggerSyncReservations()
}

func AdminReservationToggleHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		HttpError(w, http.StatusBadRequest, "invalid_request")
		return
	}

	kube.ReservationSyncEnabled = req.Enabled
	slog.Info("AUDIT: admin reservation sync toggled", "user", user.Username, "enabled", req.Enabled, "remote_addr", r.RemoteAddr)

	JsonResponse(w, map[string]any{"reservationSyncEnabled": kube.ReservationSyncEnabled})
}

func AdminExportDatabase(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	slog.Info("AUDIT: admin database export requested", "user", user.Username, "remote_addr", r.RemoteAddr)

	db := database.DB()

	// Flush WAL
	if _, err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		slog.Error("admin export: WAL checkpoint failed", "error", err)
		HttpError(w, http.StatusInternalServerError, "checkpoint_failed")
		return
	}

	f, err := os.Open(database.DBFilePath)
	if err != nil {
		slog.Error("admin export: failed to open db file", "error", err)
		HttpError(w, http.StatusInternalServerError, "file_open_failed")
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "file_stat_failed")
		return
	}

	slog.Info("AUDIT: admin database export serving", "user", user.Username, "file_size", stat.Size())

	w.Header().Set("Content-Disposition", "attachment; filename=bookings.db")
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, "bookings.db", stat.ModTime(), f)
}

func AdminImportDatabase(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	slog.Info("AUDIT: admin database import requested", "user", user.Username, "remote_addr", r.RemoteAddr)

	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	file, _, err := r.FormFile("database")
	if err != nil {
		HttpError(w, http.StatusBadRequest, "missing_database_field")
		return
	}
	defer file.Close()

	tmpFile, err := os.CreateTemp("", "bookings-import-*.db")
	if err != nil {
		HttpError(w, http.StatusInternalServerError, "temp_file_failed")
		return
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpFile, file); err != nil {
		tmpFile.Close()
		HttpError(w, http.StatusInternalServerError, "upload_copy_failed")
		return
	}
	tmpFile.Close()

	database.DBMu.Lock()
	defer database.DBMu.Unlock()

	database.Close()

	src, err := os.Open(tmpPath)
	if err != nil {
		database.OpenDB(database.DBFilePath)
		HttpError(w, http.StatusInternalServerError, "import_open_failed")
		return
	}
	dst, err := os.Create(database.DBFilePath)
	if err != nil {
		src.Close()
		database.OpenDB(database.DBFilePath)
		HttpError(w, http.StatusInternalServerError, "import_create_failed")
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		src.Close()
		dst.Close()
		database.OpenDB(database.DBFilePath)
		HttpError(w, http.StatusInternalServerError, "import_copy_failed")
		return
	}
	src.Close()
	dst.Close()

	os.Remove(database.DBFilePath + "-wal")
	os.Remove(database.DBFilePath + "-shm")

	if err := database.OpenDB(database.DBFilePath); err != nil {
		slog.Error("admin import: failed to reopen database", "error", err)
		HttpError(w, http.StatusInternalServerError, "reopen_failed")
		return
	}

	slog.Info("AUDIT: admin database imported successfully", "user", user.Username)
	kube.TriggerSyncReservations()

	JsonResponse(w, map[string]string{"status": "imported"})
}

func AdminDiscoverGPUHandler(w http.ResponseWriter, r *http.Request) {
	user := GetUser(r)
	if !user.IsAdmin {
		HttpError(w, http.StatusForbidden, "admin_required")
		return
	}

	slog.Info("AUDIT: admin GPU discovery triggered", "user", user.Username, "remote_addr", r.RemoteAddr)

	result, err := kube.RunDiscovery()
	if err != nil {
		slog.Error("admin GPU discovery failed", "error", err)
		HttpError(w, http.StatusInternalServerError, "discovery_failed: "+err.Error())
		return
	}
	if result == nil {
		HttpError(w, http.StatusNotFound, "no_gpu_nodes_found")
		return
	}

	database.SetGPUConfig(&database.GPUConfig{
		Resources:   result.Resources,
		TotalCPU:    result.TotalCPU,
		TotalMemory: result.TotalMemory,
		FlavorName:  result.FlavorName,
	})

	slog.Info("admin GPU discovery applied", "resources", len(result.Resources), "totalCPU", result.TotalCPU, "totalMemory", result.TotalMemory, "flavorName", result.FlavorName)

	JsonResponse(w, map[string]any{
		"status":      "discovered",
		"resources":   result.Resources,
		"totalCpu":    result.TotalCPU,
		"totalMemory": result.TotalMemory,
		"flavorName":  result.FlavorName,
	})
}
