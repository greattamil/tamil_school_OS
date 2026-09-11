// Command worker is the separate background-job process (PRD 8.1: "Background
// worker: Separate Go process -- PDF generation, notification dispatch, nightly
// jobs"). It claims jobs from the shared PostgreSQL table (internal/jobs) and,
// on its own timer, scans for schools whose configured absence-notification time
// has arrived (PRD 4.2.3).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	appdb "school-erp/backend/internal/db"
	"school-erp/backend/internal/fees"
	"school-erp/backend/internal/jobs"
	"school-erp/backend/internal/notify"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}

	pool, err := appdb.NewPool(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	workerID := "worker-" + hostname()
	w := jobs.NewWorker(pool, workerID)
	notify.RegisterJobHandlers(w, pool, notify.LogDispatcher{})

	log.Printf("worker %s started", workerID)

	jobTicker := time.NewTicker(2 * time.Second)
	defer jobTicker.Stop()
	// The absence-notification scan only needs to catch a school crossing its
	// configured time once; a minute of slack either side is harmless (PRD 4.2.3
	// gives a default of 11:00, not a to-the-second SLA).
	scanTicker := time.NewTicker(1 * time.Minute)
	defer scanTicker.Stop()
	// PRD 4.5.3: emergency-broadcast SMS rollover after a 3-minute unacknowledged
	// window -- checked more often than that window so no recipient waits much
	// past it, but not so often it's meaningfully different from checking on the
	// window's own cadence.
	const smsRolloverWindow = 3 * time.Minute
	smsRolloverTicker := time.NewTicker(30 * time.Second)
	defer smsRolloverTicker.Stop()
	// PRD 4.4.5: fee due/overdue reminders. A daily cadence would be the
	// normal choice for something checking date-based windows, not a fast
	// ticker -- this shorter interval is deliberately for this session's own
	// verification convenience (waiting a day to prove it fires is not
	// practical); nothing about the logic depends on the tick being fast.
	const feeReminderDueWindow = 3 * 24 * time.Hour
	const feeReminderOverdueCooldown = 7 * 24 * time.Hour
	feeReminderTicker := time.NewTicker(1 * time.Minute)
	defer feeReminderTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("worker shutting down")
			return nil
		case <-jobTicker.C:
			n, err := w.RunOnce(ctx, 20)
			if err != nil {
				log.Printf("worker: run once: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("worker: processed %d job(s)", n)
			}
		case <-scanTicker.C:
			n, err := notify.ScanAbsenceNotifications(ctx, pool)
			if err != nil {
				log.Printf("worker: absence scan: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("worker: absence scan enqueued alerts for %d school(s)", n)
			}
		case <-smsRolloverTicker.C:
			n, err := notify.ScanSMSRollover(ctx, pool, notify.LogSMSSender{}, smsRolloverWindow)
			if err != nil {
				log.Printf("worker: sms rollover scan: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("worker: sms rollover sent %d message(s)", n)
			}
		case <-feeReminderTicker.C:
			n, err := fees.ScanFeeReminders(ctx, pool, feeReminderDueWindow, feeReminderOverdueCooldown)
			if err != nil {
				log.Printf("worker: fee reminder scan: %v", err)
				continue
			}
			if n > 0 {
				log.Printf("worker: fee reminder scan sent %d reminder(s)", n)
			}
		}
	}
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "unknown"
	}
	return h
}
