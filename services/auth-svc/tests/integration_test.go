
package tests

import (
    "context"
    "testing"

    "github.com/stretchr/testify/require"
    postgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func Test_DB_Basic(t *testing.T) {
    t.Parallel()
    ctx := context.Background()
    pgC, err := postgres.RunContainer(ctx,
        postgres.WithDatabase("shop"),
        postgres.WithUsername("postgres"),
        postgres.WithPassword("postgres"),
    )
    require.NoError(t, err)
    t.Cleanup(func(){ _ = pgC.Terminate(ctx) })
    // TODO: add pgx connection, migrate schema, run CRUD
}
