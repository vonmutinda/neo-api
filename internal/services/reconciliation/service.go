package reconciliation

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"math"
	"os"
	"strconv"
	"time"

	"github.com/vonmutinda/neo/internal/domain"
	"github.com/vonmutinda/neo/internal/ledger"
	"github.com/vonmutinda/neo/internal/repository"
	nlog "github.com/vonmutinda/neo/pkg/logger"
)

// Service performs End-of-Day 3-way reconciliation between EthSwitch, Postgres, and Formance.
type Service struct {
	receipts repository.TransactionReceiptRepository
	recon    repository.ReconciliationRepository
	audit    repository.AuditRepository
	ledger   ledger.Client
}

func NewService(
	receipts repository.TransactionReceiptRepository,
	recon repository.ReconciliationRepository,
	audit repository.AuditRepository,
	ledgerClient ledger.Client,
) *Service {
	return &Service{
		receipts: receipts,
		recon:    recon,
		audit:    audit,
		ledger:   ledgerClient,
	}
}

// RunDailyReconciliation processes an EthSwitch clearing file and performs
// a 3-way match against Postgres receipts and the Formance ledger.
func (s *Service) RunDailyReconciliation(ctx context.Context, clearingFilePath string) error {
	log := nlog.FromContext(ctx)

	runDate := time.Now().Truncate(24 * time.Hour)
	fileName := clearingFilePath

	run := &domain.ReconRun{
		RunDate:          runDate,
		ClearingFileName: fileName,
	}
	if err := s.recon.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("creating recon run: %w", err)
	}

	file, err := os.Open(clearingFilePath)
	if err != nil {
		errMsg := fmt.Sprintf("could not open clearing file: %v", err)
		_ = s.recon.FinishRun(ctx, run.ID, 0, 0, &errMsg)
		return fmt.Errorf("opening clearing file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// Skip header
	if _, err := reader.Read(); err != nil {
		return fmt.Errorf("reading csv header: %w", err)
	}

	var matched, exceptions int

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Error("error reading CSV line", slog.String("error", err.Error()))
			continue
		}

		ethRef := record[0]
		amountCents := parseAmount(record[2])

		// Leg 1: Check Postgres for our internal receipt
		receipt, err := s.receipts.GetByEthSwitchReference(ctx, ethRef)
		if err != nil {
			// EthSwitch has a record we don't -- critical exception
			s.flagException(ctx, ethRef, domain.ExceptionMissingInLedger, amountCents, nil, nil, runDate, &fileName)
			exceptions++
			continue
		}

		// Leg 2: Check Formance for the actual posted transaction
		txs, err := s.ledger.GetAccountHistory(ctx, receipt.LedgerTransactionID, 1)
		if err != nil || len(txs) == 0 {
			s.flagException(ctx, ethRef, domain.ExceptionStatusMismatch, amountCents, &receipt.AmountCents, nil, runDate, &fileName)
			exceptions++
			continue
		}

		// Leg 3: Amount match
		if amountCents != receipt.AmountCents {
			diff := int64(math.Abs(float64(amountCents - receipt.AmountCents)))
			s.flagException(ctx, ethRef, domain.ExceptionAmountMismatch, amountCents, &receipt.AmountCents, &diff, runDate, &fileName)
			exceptions++
			continue
		}

		matched++
	}

	if err := s.recon.FinishRun(ctx, run.ID, matched, exceptions, nil); err != nil {
		return fmt.Errorf("finishing recon run: %w", err)
	}

	log.Info("reconciliation complete",
		slog.Int("matched", matched),
		slog.Int("exceptions", exceptions),
	)

	return nil
}

func (s *Service) flagException(ctx context.Context, ethRef string, errType domain.ExceptionType, ethAmount int64, pgAmount, diff *int64, runDate time.Time, fileName *string) {
	exc := &domain.ReconException{
		EthSwitchReference:           ethRef,
		ErrorType:                    errType,
		EthSwitchReportedAmountCents: ethAmount,
		PostgresReportedAmountCents:  pgAmount,
		AmountDifferenceCents:        diff,
		ReconRunDate:                 runDate,
		ClearingFileName:             fileName,
	}
	_ = s.recon.CreateException(ctx, exc)
	_ = s.audit.Log(ctx, &domain.AuditEntry{
		Action:       domain.AuditReconExceptionOpened,
		ActorType:    "cron",
		ResourceType: "recon_exception",
		ResourceID:   ethRef,
	})
}

func parseAmount(s string) int64 {
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return v
}
