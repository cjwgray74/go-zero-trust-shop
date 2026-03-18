package migrate

import (
    "context"
    "embed"
    "fmt"
    "sort"
    "strings"
    "time"

    "github.com/jackc/pgx/v5"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type migration struct {
    Name string
    Up   string
    Down string
}

func readMigrations() ([]migration, error) {
    entries, err := migrationsFS.ReadDir("migrations")
    if err != nil {
        return nil, err
    }

    type pair struct{ up, down string }
    files := map[string]*pair{}

    for _, e := range entries {
        if e.IsDir() {
            continue
        }
        name := e.Name()
        path := "migrations/" + name

        switch {
        case strings.HasSuffix(name, ".up.sql"):
            key := strings.TrimSuffix(name, ".up.sql")
            if files[key] == nil {
                files[key] = &pair{}
            }
            b, err := migrationsFS.ReadFile(path)
            if err != nil {
                return nil, err
            }
            files[key].up = string(b)

        case strings.HasSuffix(name, ".down.sql"):
            key := strings.TrimSuffix(name, ".down.sql")
            if files[key] == nil {
                files[key] = &pair{}
            }
            b, err := migrationsFS.ReadFile(path)
            if err != nil {
                return nil, err
            }
            files[key].down = string(b)
        }
    }

    keys := make([]string, 0, len(files))
    for k := range files {
        keys = append(keys, k)
    }
    sort.Strings(keys)

    out := make([]migration, 0, len(keys))
    for _, k := range keys {
        p := files[k]
        out = append(out, migration{Name: k, Up: p.up, Down: p.down})
    }
    return out, nil
}

func Up(ctx context.Context, conn *pgx.Conn) error {
    if _, err := conn.Exec(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations(
          name TEXT PRIMARY KEY,
          applied_at TIMESTAMPTZ NOT NULL
        );
    `); err != nil {
        return fmt.Errorf("ensure schema_migrations: %w", err)
    }

    migs, err := readMigrations()
    if err != nil {
        return fmt.Errorf("read migrations: %w", err)
    }

    for _, m := range migs {
        var exists bool
        if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, m.Name).Scan(&exists); err != nil {
            return fmt.Errorf("check migration %s: %w", m.Name, err)
        }
        if exists {
            continue
        }

        tx, err := conn.Begin(ctx)
        if err != nil {
            return fmt.Errorf("begin: %w", err)
        }
        if _, err := tx.Exec(ctx, m.Up); err != nil {
            _ = tx.Rollback(ctx)
            return fmt.Errorf("apply %s: %w", m.Name, err)
        }
        if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations(name, applied_at) VALUES($1,$2)`, m.Name, time.Now().UTC()); err != nil {
            _ = tx.Rollback(ctx)
            return fmt.Errorf("record %s: %w", m.Name, err)
        }
        if err := tx.Commit(ctx); err != nil {
            return fmt.Errorf("commit %s: %w", m.Name, err)
        }
    }
    return nil
}