package db

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/rs3-market/backend/shared/config"
)

// slowQueryThreshold is how long a statement has to take before it is
// worth a line in the log on its own.
const slowQueryThreshold = 200 * time.Millisecond

// gormLogLevel maps mysql.log_level onto GORM's levels, defaulting to
// warn.
//
// SQL logging is configured separately from the service's own log_level
// on purpose. Tying them together meant that asking a service for debug
// logs also asked GORM to print every statement it ran, and a scrape
// upserting thousands of recipes then buried its own progress under
// hundreds of thousands of query lines — the logs became unreadable
// exactly when someone was watching them. Warn still reports errors and
// slow queries, which is the part that was worth having.
func gormLogLevel(level string) logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "silent", "none", "off":
		return logger.Silent
	case "error":
		return logger.Error
	case "info", "debug", "all":
		return logger.Info
	default:
		return logger.Warn
	}
}

func Open(cfg config.MySQLConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=UTC&multiStatements=true",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName,
	)

	gormLogger := logger.New(
		log.New(os.Stdout, "", log.LstdFlags),
		logger.Config{
			SlowThreshold: slowQueryThreshold,
			LogLevel:      gormLogLevel(cfg.LogLevel),
			// A miss on a lookup is an ordinary outcome here, not
			// something to log about.
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	gdb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger:  gormLogger,
		NowFunc: func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	return gdb, nil
}
