// backup exports every table to one gzipped file and can load such a file
// into an empty schema. It is plain Go over the database connection, so it
// needs neither pg_dump nor Docker and runs the same on a laptop and in CI.
//
//	backup export <file.jsonl.gz>     read-only; DATABASE_URL (+ DB_SCHEMA)
//	backup restore <file.jsonl.gz>    into DB_SCHEMA, which must be empty of users
//	backup verify <file.jsonl.gz>     restore into a scratch schema, compare counts, drop it
package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"

	"companion/api/internal/store"
)

// tables is the load order: a table comes after everything it refers to.
// thoughts and thought_interpretations refer to each other, so thoughts are
// loaded without their "current interpretation" pointer and it is set after.
var tables = []string{
	"users", "user_preferences", "thoughts", "thought_interpretations", "decision_runs",
	"nudges", "thought_events", "llm_calls", "signals", "reviews", "kv",
}

// withIdentity lists the tables whose id is generated, so their sequences
// can be moved past the restored rows.
var withIdentity = []string{
	"users", "thoughts", "thought_interpretations", "decision_runs", "nudges", "thought_events", "llm_calls", "signals", "reviews",
}

type line struct {
	Table string          `json:"table"`
	Row   json.RawMessage `json:"row"`
}

func main() {
	if len(os.Args) != 3 {
		fail("usage: backup export|restore|verify <file.jsonl.gz>")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	url, schema := os.Getenv("DATABASE_URL"), os.Getenv("DB_SCHEMA")
	if url == "" {
		fail("DATABASE_URL must be set")
	}
	var err error
	switch os.Args[1] {
	case "export":
		err = export(ctx, url, schema, os.Args[2])
	case "restore":
		if schema == "" || schema == "public" {
			fail("restore needs DB_SCHEMA set to an empty, non-public schema; move it into place by hand once checked")
		}
		err = restore(ctx, url, schema, os.Args[2])
	case "verify":
		err = verify(ctx, url, os.Args[2])
	default:
		fail("unknown command " + os.Args[1])
	}
	if err != nil {
		fail(err.Error())
	}
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "backup:", msg)
	os.Exit(1)
}

func export(ctx context.Context, url, schema, path string) error {
	db, err := store.Open(ctx, url, schema)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Pool().BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly, IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// A table added by a migration but missing from the list above would be
	// silently left out of every backup. Refuse instead.
	rows, err := tx.Query(ctx, `select table_name from information_schema.tables
		where table_schema = current_schema() and table_type = 'BASE TABLE' and table_name <> 'goose_db_version'`)
	if err != nil {
		return err
	}
	known := map[string]bool{}
	for _, t := range tables {
		known[t] = true
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if !known[name] {
			rows.Close()
			return fmt.Errorf("table %q exists but is not in the backup list; add it to cmd/backup", name)
		}
	}
	rows.Close()

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	enc := json.NewEncoder(gz)
	total := 0
	for _, t := range tables {
		rows, err := tx.Query(ctx, fmt.Sprintf(`select to_jsonb(t) from %q t`, t))
		if err != nil {
			return fmt.Errorf("%s: %w", t, err)
		}
		n := 0
		for rows.Next() {
			var raw []byte
			if err := rows.Scan(&raw); err != nil {
				rows.Close()
				return err
			}
			if err := enc.Encode(line{Table: t, Row: raw}); err != nil {
				rows.Close()
				return err
			}
			n++
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		fmt.Printf("  %-24s %d\n", t, n)
		total += n
	}
	if err := gz.Close(); err != nil {
		return err
	}
	fmt.Printf("exported %d rows to %s\n", total, path)
	return f.Close()
}

func restore(ctx context.Context, url, schema, path string) error {
	if err := store.Migrate(ctx, url, schema); err != nil {
		return fmt.Errorf("migrate %s: %w", schema, err)
	}
	db, err := store.Open(ctx, url, schema)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Pool().Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var users int
	if err := tx.QueryRow(ctx, `select count(*) from users`).Scan(&users); err != nil {
		return err
	}
	if users > 0 {
		return fmt.Errorf("schema %q already holds data; restore only into an empty schema", schema)
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	pointers := map[string]json.RawMessage{} // thought id -> current_interpretation_id
	counts := map[string]int{}
	for sc.Scan() {
		var l line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return fmt.Errorf("bad line: %w", err)
		}
		row := l.Row
		if l.Table == "thoughts" {
			var m map[string]json.RawMessage
			if err := json.Unmarshal(row, &m); err != nil {
				return err
			}
			if p := m["current_interpretation_id"]; len(p) > 0 && string(p) != "null" {
				pointers[string(m["id"])] = p
			}
			m["current_interpretation_id"] = json.RawMessage("null")
			if row, err = json.Marshal(m); err != nil {
				return err
			}
		}
		// jsonb_populate_record maps the JSON onto the table's own row type,
		// so this one statement works for every table and every column type.
		q := fmt.Sprintf(`insert into %q overriding system value select * from jsonb_populate_record(null::%q, $1::jsonb)`, l.Table, l.Table)
		if l.Table == "kv" || l.Table == "user_preferences" {
			q = fmt.Sprintf(`insert into %q select * from jsonb_populate_record(null::%q, $1::jsonb)`, l.Table, l.Table)
		}
		if _, err := tx.Exec(ctx, q, string(row)); err != nil {
			return fmt.Errorf("%s: %w", l.Table, err)
		}
		counts[l.Table]++
	}
	if err := sc.Err(); err != nil {
		return err
	}
	for id, ptr := range pointers {
		if _, err := tx.Exec(ctx, `update thoughts set current_interpretation_id = $2::bigint where id = $1::bigint`, id, string(ptr)); err != nil {
			return fmt.Errorf("restore pointer for thought %s: %w", id, err)
		}
	}
	for _, t := range withIdentity {
		if _, err := tx.Exec(ctx, fmt.Sprintf(
			`select setval(pg_get_serial_sequence('%s', 'id'), greatest((select coalesce(max(id), 0) from %q), 1))`, t, t)); err != nil {
			return fmt.Errorf("sequence for %s: %w", t, err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	for _, t := range tables {
		fmt.Printf("  %-24s %d\n", t, counts[t])
	}
	fmt.Printf("restored into schema %q\n", schema)
	return nil
}

// verify proves a backup file can be restored: it loads the file into a
// scratch schema, checks every table has the row count the file holds, and
// drops the schema. A backup that has never been restored is a hope.
func verify(ctx context.Context, url, path string) error {
	scratch := fmt.Sprintf("restore_check_%d", time.Now().Unix())
	defer func() {
		if err := store.DropSchema(context.Background(), url, scratch); err != nil {
			fmt.Fprintln(os.Stderr, "backup: could not drop", scratch, ":", err)
		}
	}()
	want, err := countLines(path)
	if err != nil {
		return err
	}
	if err := restore(ctx, url, scratch, path); err != nil {
		return err
	}
	db, err := store.Open(ctx, url, scratch)
	if err != nil {
		return err
	}
	defer db.Close()
	for _, t := range tables {
		var got int
		if err := db.Pool().QueryRow(ctx, fmt.Sprintf(`select count(*) from %q`, t)).Scan(&got); err != nil {
			return err
		}
		if got != want[t] {
			return fmt.Errorf("%s: file has %d rows, restored %d", t, want[t], got)
		}
	}
	var dangling int
	if err := db.Pool().QueryRow(ctx, `select count(*) from thoughts t where current_interpretation_id is not null
		and not exists (select 1 from thought_interpretations i where i.id = t.current_interpretation_id)`).Scan(&dangling); err != nil {
		return err
	}
	if dangling > 0 {
		return fmt.Errorf("%d thoughts point at a missing interpretation", dangling)
	}
	fmt.Println("verified: every table restored with matching row counts")
	return nil
}

func countLines(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	sc := bufio.NewScanner(gz)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	counts := map[string]int{}
	for sc.Scan() {
		var l struct {
			Table string `json:"table"`
		}
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return nil, err
		}
		counts[l.Table]++
	}
	return counts, sc.Err()
}
