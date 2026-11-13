package handler

import (
	"context"
	"time"

	"noder/internal/db"
	"noder/internal/logger"
	"noder/internal/model"
)

// CheckAndResetTrafficCycle 检查并执行单条订阅的周期到期清零与重置 (方案 B)
// 当订阅当前时间已到达 traffic_reset_at，且整体服务未彻底过期 (ExpiresAt > now) 时触发：
// 1. 已用流量 traffic_used_bytes 彻底清零 (0)
// 2. 流量配额 traffic_quota_bytes 重置为套餐标准周期额度 cycle_quota_bytes (绝不累加旧结余)
// 3. traffic_reset_at 顺延推进到下一个周期
// 4. 若此前因超量处于 traffic_exhausted，清零后自动恢复 active 并触发 Agent 下发放行
func CheckAndResetTrafficCycle(ctx context.Context, grant *model.AccessGrant, now time.Time) bool {
	if grant == nil || grant.TrafficResetAt == nil || grant.ExpiresAt == nil {
		return false
	}
	// 尚未到达重置时间，或者已整体过期，不在此处重置
	if grant.TrafficResetAt.After(now) || !grant.ExpiresAt.After(now) {
		return false
	}

	step := grant.CycleSeconds
	if step <= 0 {
		step = 30 * 86400 // 默认 30 天兜底
	}

	// 推进 reset_at 直到下个周期或最终过期
	nextReset := *grant.TrafficResetAt
	for !nextReset.After(now) && nextReset.Before(*grant.ExpiresAt) {
		nextReset = nextReset.Add(time.Duration(step) * time.Second)
	}
	if nextReset.After(*grant.ExpiresAt) {
		nextReset = *grant.ExpiresAt
	}

	grant.TrafficResetAt = &nextReset
	grant.TrafficUsedBytes = 0 // 周期用量彻底清零！
	if grant.CycleQuotaBytes > 0 {
		grant.TrafficQuotaBytes = grant.CycleQuotaBytes // 恢复新周期的标准套餐配额
	}
	grant.TrafficUpdatedAt = &now
	grant.UpdatedAt = now

	reactivated := false
	if grant.Status == model.AccessGrantStatusTrafficExhausted {
		grant.Status = model.AccessGrantStatusActive
		reactivated = true
	}

	_, _ = db.DB.NewUpdate().Model(grant).
		Column("traffic_reset_at", "traffic_quota_bytes", "traffic_used_bytes", "traffic_updated_at", "status", "updated_at").
		WherePK().Exec(ctx)

	if reactivated {
		createDesiredDeployments(ctx, grant.CredentialID, grant.EndpointID, model.CredentialStatusActive)
		logger.InfoContext(ctx, "traffic cycle reset: reactivated subscription",
			"grant_id", grant.ID,
			"endpoint_id", grant.EndpointID,
			"new_reset_at", nextReset,
		)
	}
	return true
}

// StartTrafficCycleWorker 定时扫描并重置到达周期时刻的代理订阅 (方案 B 周期清零守护器)
func StartTrafficCycleWorker(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runCycleResetScan(ctx)
		}
	}
}

func runCycleResetScan(ctx context.Context) {
	now := time.Now().UTC()
	var dueGrants []model.AccessGrant
	err := db.DB.NewSelect().Model(&dueGrants).
		Where("traffic_reset_at IS NOT NULL AND traffic_reset_at <= ?", now).
		Where("expires_at IS NOT NULL AND expires_at > ?", now).
		Where("status <> ?", model.AccessGrantStatusRevoked).
		Scan(ctx)
	if err != nil || len(dueGrants) == 0 {
		return
	}

	for i := range dueGrants {
		CheckAndResetTrafficCycle(ctx, &dueGrants[i], now)
	}
}
