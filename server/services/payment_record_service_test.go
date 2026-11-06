package services

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParsePaymentRecordTime(t *testing.T) {
	want := time.Date(2026, time.October, 14, 15, 30, 0, 0, paymentRecordLocation())
	for _, value := range []string{"2026-10-14 15:30:00", "2026-10-14 15:30", "2026-10-14T15:30:00+08:00"} {
		got, err := parsePaymentRecordTime(value)
		if err != nil {
			t.Fatalf("parsePaymentRecordTime(%q): %v", value, err)
		}
		if !got.Equal(want) {
			t.Fatalf("parsePaymentRecordTime(%q)=%s, want %s", value, got, want)
		}
	}
	if _, err := parsePaymentRecordTime("tomorrow"); err == nil {
		t.Fatal("invalid payment time should fail")
	}
}

func TestNormalizePaymentRecordRequest(t *testing.T) {
	request, paidAt, err := normalizePaymentRecordRequest(CreatePaymentRecordRequest{
		PaymentTime: "2026-10-14 15:30:00", Payer: "  测试用户  ", Amount: 19.999,
		PaymentMethod: " Stripe ", ConsumerApp: " ShortsWave - iOS ", Remark: " 订阅支付 ",
	})
	if err != nil {
		t.Fatal(err)
	}
	if request.Payer != "测试用户" || request.Amount != 20 || request.PaymentMethod != "Stripe" || request.ConsumerApp != "ShortsWave - iOS" {
		t.Fatalf("unexpected normalized request: %#v", request)
	}
	if request.RefundStatus != refundStatusPending || request.DeclarationStatus != declarationStatusPending {
		t.Fatalf("unexpected default statuses: %#v", request)
	}
	if paidAt.IsZero() {
		t.Fatal("payment time was not parsed")
	}
	for _, invalid := range []CreatePaymentRecordRequest{
		{PaymentTime: "2026-10-14 15:30:00", Amount: 10, ConsumerApp: "ShortsWave - iOS"},
		{PaymentTime: "2026-10-14 15:30:00", Payer: "用户", Amount: 0, ConsumerApp: "ShortsWave - iOS"},
		{PaymentTime: "bad", Payer: "用户", Amount: 10, ConsumerApp: "ShortsWave - iOS"},
		{PaymentTime: "2026-10-14 15:30:00", Payer: "用户", Amount: 10},
		{PaymentTime: "2026-10-14 15:30:00", Payer: "用户", Amount: 10, ConsumerApp: "ShortsWave - iOS", RefundStatus: "退款中"},
		{PaymentTime: "2026-10-14 15:30:00", Payer: "用户", Amount: 10, ConsumerApp: "ShortsWave - iOS", DeclarationStatus: "已拒绝"},
	} {
		if _, _, err := normalizePaymentRecordRequest(invalid); err == nil {
			t.Fatalf("invalid request should fail: %#v", invalid)
		}
	}
}

func TestPaymentRecordAPITimePreservesDatabaseWallClock(t *testing.T) {
	databaseValue := time.Date(2026, time.October, 14, 15, 30, 20, 0, time.UTC)
	got := paymentRecordAPITime(databaseValue)
	if got.Location().String() != paymentRecordLocation().String() || got.Hour() != 15 || got.Minute() != 30 {
		t.Fatalf("unexpected API time: %s (%s)", got, got.Location())
	}
}

func TestPaymentRecordFeishuFields(t *testing.T) {
	record := PaymentRecord{
		ID: "payment-id", PaymentTime: time.Date(2026, time.October, 14, 9, 30, 0, 0, paymentRecordLocation()),
		Payer: "minghong", Amount: 9.99, Currency: "USD", PaymentMethod: "Stripe",
		ConsumerApp: "ShortsWave - iOS", RefundStatus: "未退款", DeclarationStatus: "未发起",
		CreatedByName: "Stroke", UpdatedAt: time.Date(2026, time.October, 14, 10, 0, 0, 0, paymentRecordLocation()),
	}
	fields := paymentRecordFeishuFields(record, "支付记录")
	if fields["平台记录ID"] != record.ID || fields["支付金额"] != record.Amount || fields["支付记录"] == "" {
		t.Fatalf("unexpected feishu fields: %#v", fields)
	}
}

func TestPaymentRecordHandlersRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/payment-records", ListPaymentRecordsHandler)
	router.POST("/payment-records", CreatePaymentRecordHandler)
	router.PUT("/payment-records/:id", UpdatePaymentRecordHandler)
	router.PATCH("/payment-records/:id/status", UpdatePaymentRecordStatusHandler)
	for _, request := range []*http.Request{
		httpRequest(t, http.MethodGet, "/payment-records"),
		httpRequest(t, http.MethodPost, "/payment-records"),
		httpRequest(t, http.MethodPut, "/payment-records/example-id"),
		httpRequest(t, http.MethodPatch, "/payment-records/example-id/status"),
	} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Fatalf("%s %s returned %d, want 401", request.Method, request.URL.Path, response.Code)
		}
	}
}

func TestPaymentQuickStatusTransitions(t *testing.T) {
	for _, test := range []struct {
		field, current, target string
		valid                  bool
	}{
		{"refund_status", "未退款", "已退款", true},
		{"refund_status", "已退款", "未退款", false},
		{"refund_status", "已退款", "已退款", false},
		{"refund_status", "未退款", "wrong", false},
		{"corporate_declaration_status", "未发起", "已发起", true},
		{"corporate_declaration_status", "未发起", "申报完成", true},
		{"corporate_declaration_status", "已发起", "未发起", true},
		{"corporate_declaration_status", "已发起", "申报完成", true},
		{"corporate_declaration_status", "申报完成", "未发起", false},
		{"corporate_declaration_status", "申报完成", "已发起", false},
		{"corporate_declaration_status", "已发起", "已发起", false},
		{"corporate_declaration_status", "未发起", "wrong", false},
		{"amount", "未退款", "已退款", false},
		{"refund_status = 1; DROP TABLE payment_records", "未退款", "已退款", false},
	} {
		err := validatePaymentStatusTransition(test.field, test.current, test.target)
		if (err == nil) != test.valid {
			t.Errorf("%s %s -> %s: %v", test.field, test.current, test.target, err)
		}
	}
}

func httpRequest(t *testing.T, method string, path string) *http.Request {
	t.Helper()
	request, err := http.NewRequest(method, path, nil)
	if err != nil {
		t.Fatal(err)
	}
	return request
}
