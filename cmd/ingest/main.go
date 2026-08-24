package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"

	"github.com/Aditya7880900936/parcelpilot-ai/internal/db"
)

const (
	excelFile = "data/ParcelPilot_Assessment_Data.xlsx"
	timezone  = "Asia/Kolkata"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("warning: .env not found, using environment variables")
	}

	ctx := context.Background()

	pool, err := db.NewPostgres(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	f, err := excelize.OpenFile(excelFile)
	if err != nil {
		log.Fatalf("open excel: %v", err)
	}
	defer f.Close()

	if err := validateREADME(f); err != nil {
		log.Fatal(err)
	}

	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		log.Fatalf("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	if err := ingestAccounts(ctx, tx, f); err != nil {
		log.Fatal(err)
	}

	if err := ingestOrders(ctx, tx, f); err != nil {
		log.Fatal(err)
	}

	if err := ingestTickets(ctx, tx, f); err != nil {
		log.Fatal(err)
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("commit transaction: %v", err)
	}

	log.Println("✓ ParcelPilot structured data ingestion completed")
}

func validateREADME(f *excelize.File) error {
	rows, err := f.GetRows("README")
	if err != nil {
		return fmt.Errorf("read README: %w", err)
	}

	var snapshotFound bool

	for _, row := range rows {
		line := strings.Join(row, " ")

		if strings.Contains(line, "Dataset snapshot") {
			snapshotFound = true

			if !strings.Contains(line, "2026-08-16 11:00") {
				return fmt.Errorf("unexpected dataset snapshot: %s", line)
			}
		}
	}

	if !snapshotFound {
		return fmt.Errorf("dataset snapshot not found in README")
	}

	log.Println("✓ Dataset snapshot validated: 2026-08-16 11:00 Asia/Kolkata")

	return nil
}

func ingestAccounts(ctx context.Context, tx pgx.Tx, f *excelize.File) error {
	rows, err := f.GetRows("accounts")
	if err != nil {
		return fmt.Errorf("read accounts: %w", err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("accounts sheet contains no data")
	}

	for i, row := range rows[1:] {
		if len(row) < 8 {
			return fmt.Errorf("accounts row %d has %d columns, expected 8", i+2, len(row))
		}

		premiumSupport, err := strconv.ParseBool(row[6])
		if err != nil {
			return fmt.Errorf("accounts row %d: invalid premium_support: %w", i+2, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO accounts (
				account_id,
				account_name,
				plan,
				status,
				csm,
				contract_file,
				premium_support,
				notes
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (account_id)
			DO UPDATE SET
				account_name = EXCLUDED.account_name,
				plan = EXCLUDED.plan,
				status = EXCLUDED.status,
				csm = EXCLUDED.csm,
				contract_file = EXCLUDED.contract_file,
				premium_support = EXCLUDED.premium_support,
				notes = EXCLUDED.notes,
				updated_at = NOW()
		`,
			row[0],
			row[1],
			row[2],
			row[3],
			nullableString(row[4]),
			nullableString(row[5]),
			premiumSupport,
			nullableString(row[7]),
		)

		if err != nil {
			return fmt.Errorf("insert account row %d: %w", i+2, err)
		}
	}

	log.Printf("✓ accounts: %d rows", len(rows)-1)

	return nil
}

func ingestOrders(ctx context.Context, tx pgx.Tx, f *excelize.File) error {
	rows, err := f.GetRows("orders")
	if err != nil {
		return fmt.Errorf("read orders: %w", err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("orders sheet contains no data")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	for i, row := range rows[1:] {
		if len(row) < 13 {
			return fmt.Errorf("orders row %d has %d columns, expected 13", i+2, len(row))
		}

		bookedAt, err := parseOptionalTime(row[4], loc)
		if err != nil {
			return fmt.Errorf("orders row %d booked_at: %w", i+2, err)
		}

		pickupStart, err := parseOptionalTime(row[5], loc)
		if err != nil {
			return fmt.Errorf("orders row %d pickup_window_start: %w", i+2, err)
		}

		pickupEnd, err := parseOptionalTime(row[6], loc)
		if err != nil {
			return fmt.Errorf("orders row %d pickup_window_end: %w", i+2, err)
		}

		pickupActual, err := parseOptionalTime(row[7], loc)
		if err != nil {
			return fmt.Errorf("orders row %d pickup_actual_at: %w", i+2, err)
		}

		cancellationRequested, err := parseOptionalTime(row[11], loc)
		if err != nil {
			return fmt.Errorf("orders row %d cancellation_requested_at: %w", i+2, err)
		}

		shipmentFee, err := strconv.ParseFloat(row[8], 64)
		if err != nil {
			return fmt.Errorf("orders row %d shipment_fee_inr: %w", i+2, err)
		}

		carrierFault, err := strconv.ParseBool(row[9])
		if err != nil {
			return fmt.Errorf("orders row %d carrier_fault: %w", i+2, err)
		}

		customerFault, err := strconv.ParseBool(row[10])
		if err != nil {
			return fmt.Errorf("orders row %d customer_fault: %w", i+2, err)
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO orders (
				order_id,
				account_id,
				carrier,
				status,
				booked_at,
				pickup_window_start,
				pickup_window_end,
				pickup_actual_at,
				shipment_fee_inr,
				carrier_fault,
				customer_fault,
				cancellation_requested_at,
				notes
			)
			VALUES (
				$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13
			)
			ON CONFLICT (order_id)
			DO UPDATE SET
				account_id = EXCLUDED.account_id,
				carrier = EXCLUDED.carrier,
				status = EXCLUDED.status,
				booked_at = EXCLUDED.booked_at,
				pickup_window_start = EXCLUDED.pickup_window_start,
				pickup_window_end = EXCLUDED.pickup_window_end,
				pickup_actual_at = EXCLUDED.pickup_actual_at,
				shipment_fee_inr = EXCLUDED.shipment_fee_inr,
				carrier_fault = EXCLUDED.carrier_fault,
				customer_fault = EXCLUDED.customer_fault,
				cancellation_requested_at = EXCLUDED.cancellation_requested_at,
				notes = EXCLUDED.notes,
				updated_at = NOW()
		`,
			row[0],
			row[1],
			row[2],
			row[3],
			bookedAt,
			pickupStart,
			pickupEnd,
			pickupActual,
			shipmentFee,
			carrierFault,
			customerFault,
			cancellationRequested,
			nullableString(row[12]),
		)

		if err != nil {
			return fmt.Errorf("insert order row %d: %w", i+2, err)
		}
	}

	log.Printf("✓ orders: %d rows", len(rows)-1)

	return nil
}

func ingestTickets(ctx context.Context, tx pgx.Tx, f *excelize.File) error {
	rows, err := f.GetRows("tickets")
	if err != nil {
		return fmt.Errorf("read tickets: %w", err)
	}

	if len(rows) < 2 {
		return fmt.Errorf("tickets sheet contains no data")
	}

	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return fmt.Errorf("load timezone: %w", err)
	}

	for i, row := range rows[1:] {
		if len(row) < 9 {
			return fmt.Errorf(
				"tickets row %d has %d columns, expected at least 9",
				i+2,
				len(row),
			)
		}

		createdAt, err := parseOptionalTime(row[2], loc)
		if err != nil {
			return fmt.Errorf("tickets row %d created_at: %w", i+2, err)
		}

		lastMessage, err := parseOptionalTime(row[8], loc)
		if err != nil {
			return fmt.Errorf(
				"tickets row %d last_customer_message_at: %w",
				i+2,
				err,
			)
		}

		// Excelize omits trailing empty cells.
		historicalResolution := ""
		if len(row) > 9 {
			historicalResolution = row[9]
		}

		_, err = tx.Exec(ctx, `
			INSERT INTO tickets (
				ticket_id,
				account_id,
				created_at,
				status,
				subject,
				description,
				channel,
				assigned_to,
				last_customer_message_at,
				historical_resolution
			)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
			ON CONFLICT (ticket_id)
			DO UPDATE SET
				account_id = EXCLUDED.account_id,
				created_at = EXCLUDED.created_at,
				status = EXCLUDED.status,
				subject = EXCLUDED.subject,
				description = EXCLUDED.description,
				channel = EXCLUDED.channel,
				assigned_to = EXCLUDED.assigned_to,
				last_customer_message_at = EXCLUDED.last_customer_message_at,
				historical_resolution = EXCLUDED.historical_resolution,
				updated_at = NOW()
		`,
			row[0],
			row[1],
			createdAt,
			row[3],
			row[4],
			row[5],
			nullableString(row[6]),
			nullableString(row[7]),
			lastMessage,
			nullableString(historicalResolution),
		)

		if err != nil {
			return fmt.Errorf("insert ticket row %d: %w", i+2, err)
		}
	}

	log.Printf("✓ tickets: %d rows", len(rows)-1)

	return nil
}

func parseOptionalTime(value string, loc *time.Location) (*time.Time, error) {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil, nil
	}

	t, err := time.ParseInLocation(
		"2006-01-02 15:04",
		value,
		loc,
	)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)

	if value == "" {
		return nil
	}

	return &value
}
