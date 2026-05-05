//go:build integration

package monkey

import (
	"context"
	"testing"
	"time"
	"trek/internal/engine/core/types"
	"trek/internal/engine/decision"
	"trek/internal/testutil"
	"trek/pkg/coordinator"
	"trek/pkg/driver/android"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMonkeyIntegration_PreflightCheck(t *testing.T) {
	driver := testutil.RequireDevice(t)

	result, err := driver.CheckEnvironment(string(android.PageTypeUIA))
	require.NoError(t, err, "环境检查失败")
	assert.True(t, result.ADBReady, "ADB 应就绪")
	assert.True(t, result.DeviceReady, "设备应就绪")
	t.Logf("环境检查: ADB=%v Device=%v PageSource=%v UIA=%v DeviceName=%s",
		result.ADBReady, result.DeviceReady, result.PageSourceReady, result.UIAReady, result.DeviceName)
}

func TestMonkeyIntegration_ShortRun(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)

	coord, err := coordinator.New(coordinator.Config{
		PackageName: pkgName,
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	require.NoError(t, err, "创建 Coordinator 失败")

	maxSteps := 5
	runner, err := NewRunner(coord, driver, Config{
		PackageName:     pkgName,
		MaxSteps:        maxSteps,
		MaxDuration:     60 * time.Second,
		StepInterval:    500 * time.Millisecond,
		StopOnCrash:     true,
		StopOnANR:       true,
		KeepStepRecords: true,
		PageSourceType:  string(android.PageTypeUIA),
	})
	require.NoError(t, err, "创建 Runner 失败")

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	require.NoError(t, err, "Runner 执行失败")
	require.NotNil(t, report, "Report 不应为 nil")

	t.Logf("运行报告: Steps=%d/%d Succeeded=%d Failed=%d StopReason=%s Duration=%dms",
		report.StepsTotal, report.StepsPlanned,
		report.StepsSucceeded, report.StepsFailed,
		report.StopReason, report.DurationMs)

	assert.Greater(t, report.StepsTotal, 0, "应至少执行一步")
	assert.NotEmpty(t, string(report.StopReason), "停止原因不应为空")

	if len(report.Records) > 0 {
		for i, rec := range report.Records {
			t.Logf("步骤 %d: page=%s action=%s duration=%dms err=%s",
				i, rec.PageName, rec.Action, rec.DurationMs, rec.Err)
		}
	}
}

func TestMonkeyIntegration_WithBlockRecovery(t *testing.T) {
	driver := testutil.RequireDevice(t)
	pkgName := testutil.DetectForegroundPackage(t, driver)

	coord, err := coordinator.New(coordinator.Config{
		PackageName: pkgName,
		Algorithm:   decision.AlgorithmReuse,
		DeviceType:  types.Phone,
	})
	require.NoError(t, err)

	enableBlockRecovery := true
	runner, err := NewRunner(coord, driver, Config{
		PackageName:            pkgName,
		MaxSteps:               10,
		MaxDuration:            90 * time.Second,
		StepInterval:           500 * time.Millisecond,
		StopOnCrash:            true,
		StopOnANR:              true,
		KeepStepRecords:        true,
		PageSourceType:         string(android.PageTypeUIA),
		EnableBlockRecovery:    &enableBlockRecovery,
		BlockNoChangeThreshold: 3,
		RecoveryCooldownSteps:  2,
	})
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	report, err := runner.Run(ctx)
	require.NoError(t, err)
	require.NotNil(t, report)

	t.Logf("恢复测试报告: Steps=%d StopReason=%s CooldownEnter=%d OutOfAppRecoveries=%d",
		report.StepsTotal, report.StopReason,
		report.RecoveryCooldownEnterCount, report.OutOfAppRecoveries)
}
