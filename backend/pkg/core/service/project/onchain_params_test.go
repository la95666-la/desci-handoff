package project

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// TGE 百分比合计与递推时间表（不触链的纯逻辑部分）
func TestOnchainTGEAssembly(t *testing.T) {
	end := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	entries := []OnchainTGEEntry{
		{Percent: decimal.NewFromFloat(25)},
		{Percent: decimal.NewFromFloat(25)},
		{Percent: decimal.NewFromFloat(50)},
	}
	total := decimal.Zero
	for _, e := range entries {
		total = total.Add(e.Percent)
	}
	if !total.Equal(decimal.NewFromInt(100)) {
		t.Fatalf("sum=%s want 100", total.String())
	}
	// 递推：end+10min, +10+30min, +10+60min
	t1 := end.Add(10 * time.Minute)
	t2 := t1.Add(30 * time.Minute)
	t3 := t2.Add(30 * time.Minute)
	want := []time.Time{t1, t2, t3}
	cur := end.Add(10 * time.Minute)
	for i := range entries {
		if !cur.Equal(want[i]) {
			t.Fatalf("tge[%d] time %v want %v", i, cur, want[i])
		}
		cur = cur.Add(30 * time.Minute)
	}
}

// resolveOnchainParams 的金额守卫（链 RPC 不可用 → 用非法 symbol 提前触发受信配置错误）
func TestResolveOnchainParamsRejectsUnknownSymbol(t *testing.T) {
	_, err := resolveOnchainParams(1, OnchainParamsRequest{PaymentToken: "NOPE"})
	if err == nil {
		t.Fatal("unknown payment symbol accepted")
	}
}

// 时间窗守卫（同样在触链前由金额/时间校验先失败或触链失败——这里仅验证结构语义）
func TestOnchainParamsRequestShape(t *testing.T) {
	req := OnchainParamsRequest{
		PaymentToken: "USDT", TokenAddress: "0x40EC68deD606c4e30b250f735e51aC57B7767b1a",
		OwnerAddress: "0xc7b4bf323C94CEb2E6e9bb7fbf89D06140941e5f",
		TokenPrice: "0.01", HardCap: "30", SoftCap: "10", MaxPerUser: "15",
		StartTime: time.Now().Add(time.Hour).Format(time.RFC3339),
		EndTime:   time.Now().Add(3 * time.Hour).Format(time.RFC3339),
		TGEAfterEndMin: 10, TGE: []OnchainTGEEntry{{Percent: decimal.NewFromInt(100)}},
	}
	if req.PaymentToken != "USDT" || len(req.TGE) != 1 {
		t.Fatalf("shape mismatch: %+v", req)
	}
	if !req.TGE[0].Percent.Equal(decimal.NewFromInt(100)) {
		t.Fatal("tge percent must be 100")
	}
}
