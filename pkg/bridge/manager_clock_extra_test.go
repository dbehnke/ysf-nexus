package bridge

import (
	"os"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
)

func Test_shouldStartNowWithDuration_CrossDayAndDescriptors(t *testing.T) {
	l := logger.NewTestLogger(os.Stdout)

	tests := []struct {
		name      string
		now       time.Time
		schedule  string
		duration  time.Duration
		wantStart bool
	}{
		{
			name:      "cross-day window (late night)",
			now:       time.Date(2025, 10, 4, 0, 1, 30, 0, time.UTC),
			schedule:  "30 23 * * * *",  // 23:00:30 every day
			duration:  90 * time.Minute, // long duration covering midnight
			wantStart: true,
		},
		{
			name:      "@hourly descriptor",
			now:       time.Date(2025, 10, 3, 15, 0, 10, 0, time.UTC),
			schedule:  "@hourly",
			duration:  5 * time.Minute,
			wantStart: true,
		},
		{
			name:      "@hourly outside",
			now:       time.Date(2025, 10, 3, 15, 6, 0, 0, time.UTC),
			schedule:  "@hourly",
			duration:  5 * time.Minute,
			wantStart: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &FakeClock{NowTime: tt.now}
			mgr := NewManagerWithClock([]config.BridgeConfig{}, nil, l, fake)

			cfg := config.BridgeConfig{Schedule: tt.schedule, Duration: tt.duration}
			gotStart, _ := mgr.shouldStartNowWithDuration(cfg)
			if gotStart != tt.wantStart {
				t.Fatalf("%s: expected start=%v got=%v", tt.name, tt.wantStart, gotStart)
			}
		})
	}
}
