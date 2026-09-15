package db

import (
	"testing"

	"gorm.io/gorm/logger"
)

func TestGormLogLevelDefaultsToWarn(t *testing.T) {
	if got := gormLogLevel(""); got != logger.Warn {
		t.Errorf("gormLogLevel(%q) = %v, want Warn", "", got)
	}
}

// An unset or misspelled level must not silence errors and slow
// queries, and must not turn on per-statement tracing either.
func TestGormLogLevelUnknownFallsBackToWarn(t *testing.T) {
	if got := gormLogLevel("verbose"); got != logger.Warn {
		t.Errorf("gormLogLevel(%q) = %v, want Warn", "verbose", got)
	}
}

func TestGormLogLevelNamedLevels(t *testing.T) {
	cases := map[string]logger.LogLevel{
		"silent": logger.Silent,
		"none":   logger.Silent,
		"off":    logger.Silent,
		"error":  logger.Error,
		"warn":   logger.Warn,
		"info":   logger.Info,
		"debug":  logger.Info,
		"all":    logger.Info,
	}
	for level, want := range cases {
		if got := gormLogLevel(level); got != want {
			t.Errorf("gormLogLevel(%q) = %v, want %v", level, got, want)
		}
	}
}

func TestGormLogLevelIgnoresCaseAndSpace(t *testing.T) {
	if got := gormLogLevel("  Silent "); got != logger.Silent {
		t.Errorf("gormLogLevel(%q) = %v, want Silent", "  Silent ", got)
	}
}

// The whole point of the split: a service running at log_level debug
// must not drag GORM into printing every statement with it.
func TestGormLogLevelIsNotDrivenByServiceLogLevel(t *testing.T) {
	var unset string
	if got := gormLogLevel(unset); got == logger.Info {
		t.Error("an unset mysql.log_level enabled statement logging")
	}
}
