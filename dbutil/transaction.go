// Copyright (c) 2023 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package dbutil

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/go-whatsapp/go-util/exerrors"
	"github.com/go-whatsapp/go-util/random"
)

var (
	ErrTxn       = errors.New("transaction")
	ErrTxnBegin  = fmt.Errorf("%w: begin", ErrTxn)
	ErrTxnCommit = fmt.Errorf("%w: commit", ErrTxn)
)

type contextKey int

const (
	ContextKeyDatabaseTransaction contextKey = iota
	ContextKeyDoTxnCallerSkip
)

func (db *Database) DoTxn(ctx context.Context, opts *sql.TxOptions, fn func(ctx context.Context) error) error {
	if ctx.Value(ContextKeyDatabaseTransaction) != nil {
		zerolog.Ctx(ctx).Trace().Msg("Already in a transaction, not creating a new one")
		return fn(ctx)
	}
	log := zerolog.Ctx(ctx).With().Str("db_txn_id", random.String(12)).Logger()
	start := time.Now()
	defer func() {
		dur := time.Since(start)
		if dur > time.Second {
			val := ctx.Value(ContextKeyDoTxnCallerSkip)
			callerSkip := 2
			if val != nil {
				callerSkip += val.(int)
			}
			log.Warn().
				Float64("duration_seconds", dur.Seconds()).
				Caller(callerSkip).
				Msg("Transaction took long")
		}
	}()
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		log.Trace().Err(err).Msg("Failed to begin transaction")
		return exerrors.NewDualError(ErrTxnBegin, err)
	}
	log.Trace().Msg("Transaction started")
	tx.noTotalLog = true
	ctx = log.WithContext(ctx)
	ctx = context.WithValue(ctx, ContextKeyDatabaseTransaction, tx)
	err = fn(ctx)
	if err != nil {
		log.Trace().Err(err).Msg("Database transaction failed, rolling back")
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			log.Warn().Err(rollbackErr).Msg("Rollback after transaction error failed")
		} else {
			log.Trace().Msg("Rollback successful")
		}
		return err
	}
	err = tx.Commit()
	if err != nil {
		log.Trace().Err(err).Msg("Commit failed")
		return exerrors.NewDualError(ErrTxnCommit, err)
	}
	log.Trace().Msg("Commit successful")
	return nil
}

func (db *Database) Conn(ctx context.Context) ContextExecable {
	if ctx == nil {
		return db
	}
	txn, ok := ctx.Value(ContextKeyDatabaseTransaction).(Transaction)
	if ok {
		return txn
	}
	return db
}
