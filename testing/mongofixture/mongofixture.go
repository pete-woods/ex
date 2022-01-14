/*
Package mongofixture will setup an isolated Mongo DB for your tests, so they don't interfere.
*/
package mongofixture

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gotest.tools/v3/assert"

	"github.com/circleci/ex/o11y"
)

type Fixture struct {
	DB   *mongo.Database
	Name string
	URI  string
}

type Connection struct {
	URI string
}

func Setup(ctx context.Context, t testing.TB, con Connection) *Fixture {
	t.Helper()
	ctx, span := o11y.StartSpan(ctx, "mongofixture: setup")
	defer span.End()

	opts := options.Client().
		ApplyURI(con.URI).
		SetAppName("test")

	client, err := mongo.Connect(ctx, opts)
	assert.Assert(t, err)

	t.Cleanup(func() {
		assert.Check(t, client.Disconnect(ctx))
	})

	name := fmt.Sprintf("%s-%s", randomSuffix(), strings.ReplaceAll(t.Name(), "/", "_"))
	name = truncate(name)
	span.AddField("name", name)

	db := client.Database(name)
	t.Cleanup(func() {
		assert.Check(t, db.Drop(ctx))
	})

	return &Fixture{
		DB:   db,
		Name: name,
		URI:  con.URI,
	}
}

func randomSuffix() string {
	bytes := make([]byte, 3)
	//#nosec:G404 - this is just a name for a test database
	if _, err := rand.Read(bytes); err != nil {
		return "not-random--i-hope-thats-ok"
	}
	return hex.EncodeToString(bytes)
}

func truncate(s string) string {
	if len(s) >= 64 {
		return s[:63]
	}
	return s
}
