//go:build tray && (linux || (darwin && cgo))

package cmd

import (
	"fmt"
	"github.com/iugamlabs/aiquokka/internal/usage"
	"math"
	"time"
)

func formatLeft(used float64) string {
	return formatPct(100-math.Min(100, math.Max(0, used))) + " left"
}

// trayFacts keeps fact-only balances and reset credit details visible in both
// native image rows and Linux text rows without changing structured reports.
func trayFacts(report *usage.Report, now time.Time) []usage.Fact {
	facts := append([]usage.Fact(nil), report.Extra...)
	rc := report.ResetCredits
	if rc == nil {
		return facts
	}
	summary := fmt.Sprintf("%d available", rc.AvailableCount)
	if rc.ApplicableAvailableCount != nil {
		summary += fmt.Sprintf(" · %d applicable now", *rc.ApplicableAvailableCount)
	}
	facts = append(facts, usage.Fact{Label: "Resets", Value: summary})
	n := 0
	for _, c := range rc.Credits {
		if c.ExpiresAt == nil {
			continue
		}
		n++
		if n <= 3 {
			facts = append(facts, usage.Fact{Label: fmt.Sprintf("Reset #%d", n), Value: "expires " + usage.HumanizeReset(*c.ExpiresAt, now)})
		}
	}
	if n > 3 {
		facts = append(facts, usage.Fact{Label: "Resets", Value: fmt.Sprintf("+%d more with expiry", n-3)})
	}
	if rc.DetailsFetched && int64(len(rc.Credits)) < rc.AvailableCount {
		facts = append(facts, usage.Fact{Label: "Reset details", Value: fmt.Sprintf("%d returned (provider reports %d)", len(rc.Credits), rc.AvailableCount)})
	}
	return facts
}
