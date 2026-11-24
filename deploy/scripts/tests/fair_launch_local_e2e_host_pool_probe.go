// LOCAL-ONLY host process probe for disposable Fair Launch MySQL transport.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type profile struct{ host, port, database, user, password string }

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "fair_launch_local_e2e_host_pool_probe: FAIL: "+format+"\n", args...)
	os.Exit(1)
}
func bounded(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}
func readProfile(path string) profile {
	b, err := os.ReadFile(path)
	if err != nil {
		fail("read profile: %v", err)
	}
	p := profile{}
	for _, line := range strings.Split(string(b), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		switch parts[0] {
		case "host":
			p.host = parts[1]
		case "port":
			p.port = parts[1]
		case "database":
			p.database = parts[1]
		case "user":
			p.user = parts[1]
		case "password":
			p.password = parts[1]
		}
	}
	if p.host != "127.0.0.1" || p.port == "" || p.database == "" || p.user == "" || p.password == "" {
		fail("invalid exact loopback profile %s", filepath.Base(path))
	}
	return p
}
func openPool(path string) (*sql.DB, profile) {
	p := readProfile(path)
	// caching_sha2_password uses its RSA public-key exchange on this exact local
	// loopback transport. Cleartext-password mode is deliberately not enabled.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?tls=false&parseTime=true&multiStatements=true&timeout=3s&readTimeout=5s&writeTimeout=5s", p.user, p.password, p.host, p.port, p.database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fail("open %s: %v", p.user, err)
	}
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(time.Minute)
	ctx, cancel := bounded(5 * time.Second)
	defer cancel()
	if err = db.PingContext(ctx); err != nil {
		fail("ping %s: %v", p.user, err)
	}
	return db, p
}
func expectDenied(db *sql.DB, label, statement string) {
	ctx, cancel := bounded(4 * time.Second)
	defer cancel()
	_, err := db.ExecContext(ctx, statement)
	if err == nil {
		fail("%s unexpectedly permitted", label)
	}
	if !strings.Contains(err.Error(), "command denied") {
		fail("%s wrong error: %v", label, err)
	}
}
func main() {
	if len(os.Args) != 2 {
		fail("usage: probe <credential-dir>")
	}
	roles := []string{"runtime", "admin", "job", "ingest"}
	pools := map[string]*sql.DB{}
	ids := map[int64]bool{}
	for _, role := range roles {
		db, p := openPool(filepath.Join(os.Args[1], "mysql-"+role+".cnf"))
		defer db.Close()
		pools[role] = db
		ctx, cancel := bounded(4 * time.Second)
		var current, database string
		var id int64
		var fk int
		err := db.QueryRowContext(ctx, "SELECT CURRENT_USER(),DATABASE(),CONNECTION_ID(),@@FOREIGN_KEY_CHECKS").Scan(&current, &database, &id, &fk)
		cancel()
		if err != nil {
			fail("identity %s: %v", role, err)
		}
		if current != p.user+"@%" || database != p.database || fk != 1 {
			fail("identity mismatch %s current=%s database=%s fk=%d", role, current, database, fk)
		}
		if ids[id] {
			fail("connection id reused across independent pools: %d", id)
		}
		ids[id] = true
	}
	job := pools["job"]
	ctx, cancel := bounded(5 * time.Second)
	tx, err := job.BeginTx(ctx, nil)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE chain_event_sync_state SET last_error='host-probe-a' WHERE id=1; UPDATE chain_event_sync_state SET last_error='host-probe-commit' WHERE id=1")
	}
	if err == nil {
		err = tx.Commit()
	} else if tx != nil {
		_ = tx.Rollback()
	}
	cancel()
	if err != nil {
		fail("multi-statement commit: %v", err)
	}
	ctx, cancel = bounded(4 * time.Second)
	var value string
	err = job.QueryRowContext(ctx, "SELECT last_error FROM chain_event_sync_state WHERE id=1").Scan(&value)
	cancel()
	if err != nil || value != "host-probe-commit" {
		fail("commit semantics value=%s err=%v", value, err)
	}
	ctx, cancel = bounded(5 * time.Second)
	tx, err = job.BeginTx(ctx, nil)
	if err == nil {
		_, err = tx.ExecContext(ctx, "UPDATE chain_event_sync_state SET last_error='host-probe-rollback' WHERE id=1")
	}
	if tx != nil {
		rbErr := tx.Rollback()
		if err == nil {
			err = rbErr
		}
	}
	cancel()
	if err != nil {
		fail("rollback transaction: %v", err)
	}
	ctx, cancel = bounded(4 * time.Second)
	err = job.QueryRowContext(ctx, "SELECT last_error FROM chain_event_sync_state WHERE id=1").Scan(&value)
	cancel()
	if err != nil || value != "host-probe-commit" {
		fail("rollback semantics value=%s err=%v", value, err)
	}

	ctx, cancel = bounded(4 * time.Second)
	conn1, err := job.Conn(ctx)
	cancel()
	if err != nil {
		fail("conn1: %v", err)
	}
	ctx, cancel = bounded(4 * time.Second)
	conn2, err := job.Conn(ctx)
	cancel()
	if err != nil {
		_ = conn1.Close()
		fail("conn2: %v", err)
	}
	closeChecked := func() {
		if err := conn2.Close(); err != nil {
			_ = conn1.Close()
			fail("close conn2: %v", err)
		}
		if err := conn1.Close(); err != nil {
			fail("close conn1: %v", err)
		}
	}
	ctx, cancel = bounded(4 * time.Second)
	_, err = conn1.ExecContext(ctx, "SET SESSION innodb_lock_wait_timeout=5")
	cancel()
	if err != nil {
		closeChecked()
		fail("lock timeout conn1: %v", err)
	}
	ctx, cancel = bounded(4 * time.Second)
	_, err = conn2.ExecContext(ctx, "SET SESSION innodb_lock_wait_timeout=5")
	cancel()
	if err != nil {
		closeChecked()
		fail("lock timeout conn2: %v", err)
	}
	ctx, cancel = bounded(5 * time.Second)
	tx1, err := conn1.BeginTx(ctx, nil)
	if err == nil {
		_, err = tx1.ExecContext(ctx, "UPDATE chain_event_sync_state SET last_error='host-probe-lock' WHERE id=1")
	}
	if err == nil {
		err = tx1.Commit()
	} else if tx1 != nil {
		_ = tx1.Rollback()
	}
	cancel()
	if err != nil {
		closeChecked()
		fail("seed lock row: %v", err)
	}
	ownerCtx, ownerCancel := bounded(8 * time.Second)
	tx1, err = conn1.BeginTx(ownerCtx, nil)
	if err == nil {
		var ownerID int64
		err = tx1.QueryRowContext(ownerCtx, "SELECT id FROM chain_event_sync_state WHERE id=1 FOR UPDATE").Scan(&ownerID)
	}
	if err != nil {
		if tx1 != nil {
			_ = tx1.Rollback()
		}
		ownerCancel()
		closeChecked()
		fail("select for update owner: %v", err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		waitCtx, waitCancel := bounded(6 * time.Second)
		defer waitCancel()
		tx2, e := conn2.BeginTx(waitCtx, nil)
		if e != nil {
			done <- e
			return
		}
		close(started)
		var id int64
		e = tx2.QueryRowContext(waitCtx, "SELECT id FROM chain_event_sync_state WHERE id=1 FOR UPDATE").Scan(&id)
		if e == nil {
			e = tx2.Commit()
		} else {
			_ = tx2.Rollback()
		}
		done <- e
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		_ = tx1.Rollback()
		ownerCancel()
		closeChecked()
		fail("FOR UPDATE waiter did not start")
	}
	select {
	case e := <-done:
		_ = tx1.Rollback()
		ownerCancel()
		closeChecked()
		fail("FOR UPDATE did not block: %v", e)
	case <-time.After(500 * time.Millisecond):
	}
	err = tx1.Commit()
	ownerCancel()
	if err != nil {
		closeChecked()
		fail("release lock: %v", err)
	}
	select {
	case e := <-done:
		if e != nil {
			closeChecked()
			fail("FOR UPDATE waiter: %v", e)
		}
	case <-time.After(6 * time.Second):
		closeChecked()
		fail("FOR UPDATE waiter timeout")
	}
	// Return both checked-out connections before using the MaxOpenConns=2 pool.
	closeChecked()

	ctx, cancel = bounded(4 * time.Second)
	_, err = job.ExecContext(ctx, "UPDATE source_materialization_cursor SET last_error=last_error WHERE 1=0")
	cancel()
	if err != nil {
		fail("local_job permitted update failed: %v", err)
	}
	for _, role := range []string{"runtime", "admin", "ingest"} {
		expectDenied(pools[role], role+" protected update", "UPDATE source_materialization_cursor SET last_error=last_error WHERE 1=0")
	}
	ctx, cancel = bounded(4 * time.Second)
	var fk int
	err = job.QueryRowContext(ctx, "SELECT @@FOREIGN_KEY_CHECKS").Scan(&fk)
	cancel()
	if err != nil || fk != 1 {
		fail("FOREIGN_KEY_CHECKS final=%d err=%v", fk, err)
	}
	fmt.Printf("fair_launch_local_e2e_host_pool_probe: PASS roles=4 distinct_connections=%d commit=1 rollback=1 for_update_blocked=1 policy=1 foreign_key_checks=1\n", len(ids))
}
