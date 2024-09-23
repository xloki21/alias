package migrations

import (
	"embed"
	"github.com/golang-migrate/migrate/v4"
	mg "github.com/golang-migrate/migrate/v4/database/mongodb"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.mongodb.org/mongo-driver/mongo"
)

//go:embed mongodb/*.json
var MongoDBFilesFS embed.FS

func CreateMongoDBMigrator(client *mongo.Client, dbName string) (*migrate.Migrate, error) {
	dbDriver, err := mg.WithInstance(client, &mg.Config{
		DatabaseName: dbName,
	})
	if err != nil {
		return nil, err
	}

	sourceDriver, err := iofs.New(MongoDBFilesFS, "mongodb")
	if err != nil {
		return nil, err
	}

	migrator, err := migrate.NewWithInstance("base migrations", sourceDriver, "appdb", dbDriver)
	if err != nil {
		return nil, err
	}
	return migrator, nil
}
