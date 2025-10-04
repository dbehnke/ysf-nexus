package bridge

import (
	"os"
	"testing"
	"time"

	"github.com/dbehnke/ysf-nexus/pkg/config"
	"github.com/dbehnke/ysf-nexus/pkg/logger"
	"github.com/robfig/cron/v3"
)

func Test_shouldStartNowWithDuration_Table(t *testing.T) {
	l := logger.NewTestLogger(os.Stdout)

	tests := []struct {
		name      string
		now       time.Time
		schedule  string
		duration  time.Duration
		wantStart bool
	}{
		{
			name:      "inside window",
			now:       time.Date(2025, 10, 3, 12, 0, 5, 0, time.UTC),
			schedule:  "5 * * * * *", // second 5 of every minute
			duration:  10 * time.Second,
			wantStart: true,
		},
		{
			name:      "outside window",
			now:       time.Date(2025, 10, 3, 12, 1, 30, 0, time.UTC),
			schedule:  "0 * * * * *", // at 0 seconds each minute
			duration:  5 * time.Second,
			wantStart: false,
		},
		{
			name:      "edge of window",
			now:       time.Date(2025, 10, 3, 12, 0, 14, 0, time.UTC),
			schedule:  "5 * * * * *",
			duration:  10 * time.Second,
			wantStart: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &FakeClock{NowTime: tt.now}
			mgr := NewManagerWithClock([]config.BridgeConfig{}, nil, l, fake)

			cfg := config.BridgeConfig{Schedule: tt.schedule, Duration: tt.duration}
			gotStart, remaining := mgr.shouldStartNowWithDuration(cfg)
			if gotStart != tt.wantStart {
				// Diagnostic: compute schedule occurrences
				parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
				schedule, perr := parser.Parse(tt.schedule)
				if perr != nil {
					t.Fatalf("failed to parse schedule: %v", perr)
				}
				startPoint := tt.now.Add(-7 * 24 * time.Hour)
				candidate := schedule.Next(startPoint)
				nextFromNow := schedule.Next(tt.now)
				t.Fatalf("%s: expected start=%v got=%v; now=%v candidateNext=%v nextFromNow=%v remaining=%v",
					tt.name, tt.wantStart, gotStart, tt.now, candidate, nextFromNow, remaining)
			}
		})
	}
}
