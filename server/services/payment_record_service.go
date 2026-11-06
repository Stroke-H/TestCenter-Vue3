package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	defaultPaymentPageSize    = 20
	maxPaymentPageSize        = 100
	virtualCardInitialBalance = 500.00
	refundStatusPending       = "未退款"
	refundStatusCompleted     = "已退款"
	declarationStatusPending  = "未发起"
	declarationStatusStarted  = "已发起"
	declarationStatusDone     = "申报完成"
	feishuSyncStatusPending   = "未同步"
	feishuSyncStatusSuccess   = "已同步"
	feishuSyncStatusFailed    = "同步失败"
)

type PaymentRecord struct {
	ID                string     `json:"id"`
	PaymentTime       time.Time  `json:"payment_time"`
	Payer             string     `json:"payer"`
	Amount            float64    `json:"amount"`
	Currency          string     `json:"currency"`
	PaymentMethod     string     `json:"payment_method"`
	ConsumerApp       string     `json:"consumer_app"`
	RefundStatus      string     `json:"refund_status"`
	DeclarationStatus string     `json:"corporate_declaration_status"`
	Remark            string     `json:"remark"`
	CreatedByID       string     `json:"created_by_id"`
	CreatedByName     string     `json:"created_by_name"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	FeishuRecordID    string     `json:"feishu_record_id"`
	FeishuSyncStatus  string     `json:"feishu_sync_status"`
	FeishuSyncError   string     `json:"feishu_sync_error"`
	FeishuSyncedAt    *time.Time `json:"feishu_synced_at,omitempty"`
}

type CreatePaymentRecordRequest struct {
	PaymentTime       string  `json:"payment_time"`
	Payer             string  `json:"payer"`
	Amount            float64 `json:"amount"`
	PaymentMethod     string  `json:"payment_method"`
	ConsumerApp       string  `json:"consumer_app"`
	RefundStatus      string  `json:"refund_status"`
	DeclarationStatus string  `json:"corporate_declaration_status"`
	Remark            string  `json:"remark"`
}

type PaymentRecordSummary struct {
	Count              int     `json:"count"`
	TotalAmount        float64 `json:"total_amount"`
	CurrentMonthCount  int     `json:"current_month_count"`
	CurrentMonthAmount float64 `json:"current_month_amount"`
	AvailableBalance   float64 `json:"available_balance"`
}

type paymentRecordListQuery struct {
	Search                     string
	PaymentMethod              string
	RefundStatus               string
	CorporateDeclarationStatus string
	FeishuSyncStatus           string
	DateStart                  string
	DateEnd                    string
	Page                       int
	PageSize                   int
}

func paymentRecordLocation() *time.Location {
	if location, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		return location
	}
	return time.FixedZone("Asia/Shanghai", 8*60*60)
}

// MySQL DATETIME has no timezone. The driver may scan it in UTC, so preserve
// the stored wall-clock components and attach the platform's business timezone.
func paymentRecordAPITime(value time.Time) time.Time {
	if value.IsZero() {
		return value
	}
	return time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), value.Minute(), value.Second(), value.Nanosecond(), paymentRecordLocation())
}

func ensurePaymentRecordsTable(ctx context.Context) error {
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS payment_records (
			id VARCHAR(64) NOT NULL,
			payment_time DATETIME NOT NULL,
			payer VARCHAR(255) NOT NULL,
			amount DECIMAL(14,2) NOT NULL,
			currency VARCHAR(8) NOT NULL DEFAULT 'USD',
			payment_method VARCHAR(64) NOT NULL DEFAULT '',
			consumer_app VARCHAR(255) NOT NULL DEFAULT '',
			refund_status VARCHAR(16) NOT NULL DEFAULT '未退款',
			corporate_declaration_status VARCHAR(32) NOT NULL DEFAULT '未发起',
			remark TEXT NOT NULL,
			created_by_id VARCHAR(128) NOT NULL,
			created_by_name VARCHAR(255) NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			feishu_record_id VARCHAR(128) NOT NULL DEFAULT '',
			feishu_sync_status VARCHAR(32) NOT NULL DEFAULT '未同步',
			feishu_sync_error VARCHAR(1000) NOT NULL DEFAULT '',
			feishu_synced_at DATETIME NULL,
			PRIMARY KEY (id),
			INDEX idx_payment_records_time (payment_time),
			INDEX idx_payment_records_payer (payer),
			INDEX idx_payment_records_creator (created_by_id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
	`)
	if err != nil {
		return err
	}
	// Compatibility for databases where payment_records was created before
	// consumer_app replaced transaction_no.
	var consumerAppColumnCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_records' AND COLUMN_NAME = 'consumer_app'`).Scan(&consumerAppColumnCount); err != nil {
		return err
	}
	if consumerAppColumnCount == 0 {
		if _, err = db.ExecContext(ctx, `ALTER TABLE payment_records
			ADD COLUMN consumer_app VARCHAR(255) NOT NULL DEFAULT '' AFTER payment_method`); err != nil {
			return err
		}
	}
	var transactionNoColumnCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_records' AND COLUMN_NAME = 'transaction_no'`).Scan(&transactionNoColumnCount); err != nil {
		return err
	}
	if transactionNoColumnCount > 0 {
		_, err = db.ExecContext(ctx, `UPDATE payment_records
			SET consumer_app = transaction_no
			WHERE consumer_app = '' AND transaction_no <> ''`)
		if err != nil {
			return err
		}
	}
	for _, migration := range []struct {
		name       string
		definition string
	}{
		{name: "refund_status", definition: "VARCHAR(16) NOT NULL DEFAULT '未退款' AFTER consumer_app"},
		{name: "corporate_declaration_status", definition: "VARCHAR(32) NOT NULL DEFAULT '未发起' AFTER refund_status"},
		{name: "feishu_record_id", definition: "VARCHAR(128) NOT NULL DEFAULT '' AFTER updated_at"},
		{name: "feishu_sync_status", definition: "VARCHAR(32) NOT NULL DEFAULT '未同步' AFTER feishu_record_id"},
		{name: "feishu_sync_error", definition: "VARCHAR(1000) NOT NULL DEFAULT '' AFTER feishu_sync_status"},
		{name: "feishu_synced_at", definition: "DATETIME NULL AFTER feishu_sync_error"},
	} {
		var columnCount int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = 'payment_records' AND COLUMN_NAME = ?`, migration.name).Scan(&columnCount); err != nil {
			return err
		}
		if columnCount == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE payment_records ADD COLUMN "+migration.name+" "+migration.definition); err != nil {
				return err
			}
		}
	}
	return nil
}

func parsePaymentRecordTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, fmt.Errorf("请选择支付时间")
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed.In(paymentRecordLocation()), nil
	}
	for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
		if parsed, err := time.ParseInLocation(layout, value, paymentRecordLocation()); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("支付时间格式不正确")
}

func normalizePaymentRecordRequest(request CreatePaymentRecordRequest) (CreatePaymentRecordRequest, time.Time, error) {
	request.Payer = strings.TrimSpace(request.Payer)
	request.PaymentMethod = strings.TrimSpace(request.PaymentMethod)
	request.ConsumerApp = strings.TrimSpace(request.ConsumerApp)
	request.RefundStatus = strings.TrimSpace(request.RefundStatus)
	request.DeclarationStatus = strings.TrimSpace(request.DeclarationStatus)
	request.Remark = strings.TrimSpace(request.Remark)
	if request.RefundStatus == "" {
		request.RefundStatus = refundStatusPending
	}
	if request.DeclarationStatus == "" {
		request.DeclarationStatus = declarationStatusPending
	}
	if request.Payer == "" {
		return request, time.Time{}, fmt.Errorf("请填写支付人")
	}
	if len([]rune(request.Payer)) > 255 {
		return request, time.Time{}, fmt.Errorf("支付人不能超过 255 个字符")
	}
	if request.ConsumerApp == "" {
		return request, time.Time{}, fmt.Errorf("请选择消费应用")
	}
	if math.IsNaN(request.Amount) || math.IsInf(request.Amount, 0) || request.Amount <= 0 {
		return request, time.Time{}, fmt.Errorf("支付金额必须大于 0")
	}
	request.Amount = math.Round(request.Amount*100) / 100
	if request.Amount > 999999999999.99 {
		return request, time.Time{}, fmt.Errorf("支付金额超出支持范围")
	}
	if len([]rune(request.PaymentMethod)) > 64 || len([]rune(request.ConsumerApp)) > 255 {
		return request, time.Time{}, fmt.Errorf("支付方式或消费应用过长")
	}
	if request.RefundStatus != refundStatusPending && request.RefundStatus != refundStatusCompleted {
		return request, time.Time{}, fmt.Errorf("退款状态不正确")
	}
	if request.DeclarationStatus != declarationStatusPending && request.DeclarationStatus != declarationStatusStarted && request.DeclarationStatus != declarationStatusDone {
		return request, time.Time{}, fmt.Errorf("对公支付申报状态不正确")
	}
	if len([]rune(request.Remark)) > 2000 {
		return request, time.Time{}, fmt.Errorf("备注不能超过 2000 个字符")
	}
	paymentTime, err := parsePaymentRecordTime(request.PaymentTime)
	return request, paymentTime, err
}

func paymentRecordPageValue(value string, fallback int) int {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || parsed < 1 {
		return fallback
	}
	return parsed
}

func parsePaymentRecordListQuery(c *gin.Context) paymentRecordListQuery {
	pageSize := paymentRecordPageValue(c.Query("page_size"), defaultPaymentPageSize)
	if pageSize > maxPaymentPageSize {
		pageSize = maxPaymentPageSize
	}
	return paymentRecordListQuery{
		Search:                     strings.TrimSpace(c.Query("search")),
		PaymentMethod:              strings.TrimSpace(c.Query("payment_method")),
		RefundStatus:               strings.TrimSpace(c.Query("refund_status")),
		CorporateDeclarationStatus: strings.TrimSpace(c.Query("corporate_declaration_status")),
		FeishuSyncStatus:           strings.TrimSpace(c.Query("feishu_sync_status")),
		DateStart:                  strings.TrimSpace(c.Query("date_start")),
		DateEnd:                    strings.TrimSpace(c.Query("date_end")),
		Page:                       paymentRecordPageValue(c.Query("page"), 1),
		PageSize:                   pageSize,
	}
}

func buildPaymentRecordWhere(query paymentRecordListQuery) (string, []any) {
	conditions := []string{"1=1"}
	args := make([]any, 0, 8)
	if query.Search != "" {
		keyword := "%" + query.Search + "%"
		conditions = append(conditions, "(payer LIKE ? OR consumer_app LIKE ? OR remark LIKE ?)")
		args = append(args, keyword, keyword, keyword)
	}
	if query.PaymentMethod != "" {
		conditions = append(conditions, "payment_method = ?")
		args = append(args, query.PaymentMethod)
	}
	if query.RefundStatus != "" {
		conditions = append(conditions, "refund_status = ?")
		args = append(args, query.RefundStatus)
	}
	if query.CorporateDeclarationStatus != "" {
		conditions = append(conditions, "corporate_declaration_status = ?")
		args = append(args, query.CorporateDeclarationStatus)
	}
	if query.FeishuSyncStatus != "" {
		conditions = append(conditions, "feishu_sync_status = ?")
		args = append(args, query.FeishuSyncStatus)
	}
	if query.DateStart != "" {
		conditions = append(conditions, "payment_time >= ?")
		args = append(args, query.DateStart+" 00:00:00")
	}
	if query.DateEnd != "" {
		conditions = append(conditions, "payment_time < DATE_ADD(?, INTERVAL 1 DAY)")
		args = append(args, query.DateEnd+" 00:00:00")
	}
	return strings.Join(conditions, " AND "), args
}

func ListPaymentRecordsHandler(c *gin.Context) {
	if _, err := CurrentUserFromRequest(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := ensurePaymentRecordsTable(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	query := parsePaymentRecordListQuery(c)
	where, args := buildPaymentRecordWhere(query)
	var total int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM payment_records WHERE "+where, args...).Scan(&total); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	listArgs := append(append([]any{}, args...), query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := db.QueryContext(ctx, `SELECT id, payment_time, payer, amount, currency,
		payment_method, consumer_app, refund_status, corporate_declaration_status, remark,
		created_by_id, created_by_name, created_at, updated_at,
		feishu_record_id, feishu_sync_status, feishu_sync_error, feishu_synced_at
		FROM payment_records WHERE `+where+` ORDER BY payment_time DESC, created_at DESC LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()
	items := make([]PaymentRecord, 0)
	for rows.Next() {
		var item PaymentRecord
		var syncedAt sql.NullTime
		if err := rows.Scan(&item.ID, &item.PaymentTime, &item.Payer, &item.Amount, &item.Currency,
			&item.PaymentMethod, &item.ConsumerApp, &item.RefundStatus, &item.DeclarationStatus,
			&item.Remark, &item.CreatedByID, &item.CreatedByName,
			&item.CreatedAt, &item.UpdatedAt, &item.FeishuRecordID, &item.FeishuSyncStatus,
			&item.FeishuSyncError, &syncedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		item.PaymentTime = paymentRecordAPITime(item.PaymentTime)
		item.CreatedAt = paymentRecordAPITime(item.CreatedAt)
		item.UpdatedAt = paymentRecordAPITime(item.UpdatedAt)
		if syncedAt.Valid {
			value := paymentRecordAPITime(syncedAt.Time)
			item.FeishuSyncedAt = &value
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	monthStart := time.Now().In(paymentRecordLocation())
	monthStart = time.Date(monthStart.Year(), monthStart.Month(), 1, 0, 0, 0, 0, paymentRecordLocation())
	var summary PaymentRecordSummary
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(amount), 0),
		COALESCE(SUM(CASE WHEN payment_time >= ? THEN 1 ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN payment_time >= ? THEN amount ELSE 0 END), 0),
		? - COALESCE(SUM(CASE WHEN refund_status <> ? THEN amount ELSE 0 END), 0)
		FROM payment_records`, monthStart, monthStart, virtualCardInitialBalance, refundStatusCompleted).Scan(
		&summary.Count, &summary.TotalAmount, &summary.CurrentMonthCount, &summary.CurrentMonthAmount,
		&summary.AvailableBalance); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items, "total": total, "page": query.Page, "page_size": query.PageSize, "summary": summary,
	})
}

func CreatePaymentRecordHandler(c *gin.Context) {
	user, err := CurrentUserFromRequest(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
		return
	}
	var request CreatePaymentRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支付记录格式不正确"})
		return
	}
	request, paymentTime, err := normalizePaymentRecordRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := ensurePaymentRecordsTable(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().In(paymentRecordLocation())
	creatorName := strings.TrimSpace(user.Nickname)
	if creatorName == "" {
		creatorName = user.Username
	}
	record := PaymentRecord{
		ID: uuid.New().String(), PaymentTime: paymentTime, Payer: request.Payer,
		Amount: request.Amount, Currency: "USD", PaymentMethod: request.PaymentMethod,
		ConsumerApp: request.ConsumerApp, RefundStatus: request.RefundStatus,
		DeclarationStatus: request.DeclarationStatus, Remark: request.Remark,
		CreatedByID: user.ID, CreatedByName: creatorName, CreatedAt: now, UpdatedAt: now,
		FeishuSyncStatus: feishuSyncStatusPending,
	}
	_, err = db.ExecContext(ctx, `INSERT INTO payment_records
		(id, payment_time, payer, amount, currency, payment_method, consumer_app, refund_status,
		 corporate_declaration_status, remark,
		 created_by_id, created_by_name, created_at, updated_at, feishu_sync_status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, record.ID, record.PaymentTime, record.Payer,
		record.Amount, record.Currency, record.PaymentMethod, record.ConsumerApp, record.RefundStatus,
		record.DeclarationStatus, record.Remark,
		record.CreatedByID, record.CreatedByName, record.CreatedAt, record.UpdatedAt, record.FeishuSyncStatus)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer syncCancel()
	syncResult, syncErr := syncPaymentRecordToFeishu(syncCtx, record)
	applyPaymentRecordFeishuSyncResult(record.ID, syncResult, syncErr)
	if syncErr != nil {
		record.FeishuSyncStatus = feishuSyncStatusFailed
		record.FeishuSyncError = syncErr.Error()
		log.Printf("[PaymentRecordFeishuSync] record %s failed: %v", record.ID, syncErr)
	} else {
		nowSynced := time.Now().In(paymentRecordLocation())
		record.FeishuRecordID = syncResult.RecordID
		record.FeishuSyncStatus = feishuSyncStatusSuccess
		record.FeishuSyncedAt = &nowSynced
	}
	c.JSON(http.StatusCreated, gin.H{"record": record, "feishu_sync_success": syncErr == nil})
}

func UpdatePaymentRecordHandler(c *gin.Context) {
	if _, err := CurrentUserFromRequest(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
		return
	}
	recordID := strings.TrimSpace(c.Param("id"))
	if recordID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支付记录 ID 不能为空"})
		return
	}
	var request CreatePaymentRecordRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "支付记录格式不正确"})
		return
	}
	request, paymentTime, err := normalizePaymentRecordRequest(request)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()
	if err := ensurePaymentRecordsTable(ctx); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().In(paymentRecordLocation())
	result, err := db.ExecContext(ctx, `UPDATE payment_records SET payment_time = ?, payer = ?, amount = ?,
		payment_method = ?, consumer_app = ?, refund_status = ?, corporate_declaration_status = ?,
		remark = ?, updated_at = ? WHERE id = ?`, paymentTime, request.Payer, request.Amount,
		request.PaymentMethod, request.ConsumerApp, request.RefundStatus, request.DeclarationStatus,
		request.Remark, now, recordID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "支付记录不存在"})
		return
	}
	var record PaymentRecord
	var syncedAt sql.NullTime
	err = db.QueryRowContext(ctx, `SELECT id, payment_time, payer, amount, currency, payment_method,
		consumer_app, refund_status, corporate_declaration_status, remark, created_by_id,
		created_by_name, created_at, updated_at, feishu_record_id, feishu_sync_status,
		feishu_sync_error, feishu_synced_at FROM payment_records WHERE id = ?`, recordID).Scan(
		&record.ID, &record.PaymentTime, &record.Payer, &record.Amount, &record.Currency,
		&record.PaymentMethod, &record.ConsumerApp, &record.RefundStatus, &record.DeclarationStatus,
		&record.Remark, &record.CreatedByID, &record.CreatedByName, &record.CreatedAt, &record.UpdatedAt,
		&record.FeishuRecordID, &record.FeishuSyncStatus, &record.FeishuSyncError, &syncedAt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer syncCancel()
	syncResult, syncErr := syncPaymentRecordToFeishu(syncCtx, record)
	applyPaymentRecordFeishuSyncResult(record.ID, syncResult, syncErr)
	if syncErr != nil {
		log.Printf("[PaymentRecordFeishuSync] record %s update failed: %v", record.ID, syncErr)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "feishu_sync_success": syncErr == nil})
}

func applyPaymentRecordFeishuSyncResult(recordID string, result paymentRecordFeishuResult, syncErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	db, _, err := DatabaseManager.DB(ctx)
	if err != nil {
		log.Printf("[PaymentRecordFeishuSync] update local status failed: %v", err)
		return
	}
	if syncErr != nil {
		errorText := syncErr.Error()
		if len([]rune(errorText)) > 1000 {
			errorText = string([]rune(errorText)[:1000])
		}
		if _, err = db.ExecContext(ctx, `UPDATE payment_records SET feishu_sync_status = ?,
			feishu_sync_error = ? WHERE id = ?`, feishuSyncStatusFailed, errorText, recordID); err != nil {
			log.Printf("[PaymentRecordFeishuSync] persist failed status failed: %v", err)
		}
		return
	}
	_, err = db.ExecContext(ctx, `UPDATE payment_records SET feishu_record_id = ?, feishu_sync_status = ?,
		feishu_sync_error = '', feishu_synced_at = ? WHERE id = ?`, result.RecordID,
		feishuSyncStatusSuccess, time.Now().In(paymentRecordLocation()), recordID)
	if err != nil {
		log.Printf("[PaymentRecordFeishuSync] persist success status failed: %v", err)
	}
}
