package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type paymentRecordStatusRequest struct {
	Field          string `json:"field"`
	Status         string `json:"status"`
	ExpectedStatus string `json:"expected_status"`
}

func validatePaymentStatusTransition(field, current, target string) error {
	switch field {
	case "refund_status":
		if current != refundStatusPending || target != refundStatusCompleted {
			return fmt.Errorf("快捷退款仅支持从未退款变更为已退款")
		}
	case "corporate_declaration_status":
		if current != declarationStatusPending && current != declarationStatusStarted {
			return fmt.Errorf("申报已完成，请打开流水详情查看")
		}
		if target == current || (target != declarationStatusPending && target != declarationStatusStarted && target != declarationStatusDone) {
			return fmt.Errorf("申报状态不正确")
		}
	default:
		return fmt.Errorf("不支持的状态字段")
	}
	return nil
}

// Update one status only; never overwrite amounts, currencies or other fields
// with an older copy of a table row. Feishu writes use the existing sync path.
func UpdatePaymentRecordStatusHandler(c *gin.Context) {
	if _, err := CurrentUserFromRequest(c); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录状态已失效，请重新登录"})
		return
	}
	id := strings.TrimSpace(c.Param("id"))
	var request paymentRecordStatusRequest
	if id == "" || c.ShouldBindJSON(&request) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "状态更新参数不正确"})
		return
	}
	if err := validatePaymentStatusTransition(request.Field, request.ExpectedStatus, request.Status); err != nil {
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
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer tx.Rollback()
	var record PaymentRecord
	var syncedAt sql.NullTime
	err = tx.QueryRowContext(ctx, `SELECT id, payment_time, payer, amount, currency, payment_method,
		consumer_app, refund_status, corporate_declaration_status, remark, created_by_id,
		created_by_name, created_at, updated_at, feishu_record_id, feishu_sync_status,
		feishu_sync_error, feishu_synced_at FROM payment_records WHERE id = ? FOR UPDATE`, id).Scan(
		&record.ID, &record.PaymentTime, &record.Payer, &record.Amount, &record.Currency, &record.PaymentMethod,
		&record.ConsumerApp, &record.RefundStatus, &record.DeclarationStatus, &record.Remark, &record.CreatedByID,
		&record.CreatedByName, &record.CreatedAt, &record.UpdatedAt, &record.FeishuRecordID, &record.FeishuSyncStatus,
		&record.FeishuSyncError, &syncedAt)
	if err == sql.ErrNoRows {
		c.JSON(http.StatusNotFound, gin.H{"error": "支付记录不存在"})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	current := record.RefundStatus
	if request.Field == "corporate_declaration_status" {
		current = record.DeclarationStatus
	}
	if current != request.ExpectedStatus {
		c.JSON(http.StatusConflict, gin.H{"error": "该状态已发生变化，请刷新后重试"})
		return
	}
	if err := validatePaymentStatusTransition(request.Field, current, request.Status); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Field is restricted to the two hard-coded names above, not arbitrary SQL.
	now := time.Now().In(paymentRecordLocation())
	_, err = tx.ExecContext(ctx, "UPDATE payment_records SET "+request.Field+" = ?, updated_at = ?, feishu_sync_status = ?, feishu_sync_error = '' WHERE id = ?", request.Status, now, feishuSyncStatusPending, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if err = tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if request.Field == "refund_status" {
		record.RefundStatus = request.Status
	} else {
		record.DeclarationStatus = request.Status
	}
	record.PaymentTime = paymentRecordAPITime(record.PaymentTime)
	record.CreatedAt = paymentRecordAPITime(record.CreatedAt)
	record.UpdatedAt = now
	if syncedAt.Valid {
		value := paymentRecordAPITime(syncedAt.Time)
		record.FeishuSyncedAt = &value
	}
	syncCtx, syncCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer syncCancel()
	result, syncErr := syncPaymentRecordToFeishu(syncCtx, record)
	applyPaymentRecordFeishuSyncResult(id, result, syncErr)
	if syncErr != nil {
		record.FeishuSyncStatus, record.FeishuSyncError = feishuSyncStatusFailed, syncErr.Error()
		log.Printf("[PaymentRecordFeishuSync] record %s status update failed: %v", id, syncErr)
	} else {
		record.FeishuSyncStatus, record.FeishuSyncError, record.FeishuRecordID = feishuSyncStatusSuccess, "", result.RecordID
		value := time.Now().In(paymentRecordLocation())
		record.FeishuSyncedAt = &value
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "record": record, "feishu_sync_success": syncErr == nil})
}
