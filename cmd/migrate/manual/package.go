// Put new manual migrations as function into this package. Never change or remove existing functions.
package manual

import (
	log "github.com/osbuild/logging/pkg/logrus"
)

type Migration struct {
	Name string
	Func func() error
}

var migrations []Migration

func registerMigration(name string, f func() error) {
	migrations = append(migrations, Migration{Name: name, Func: f})
}

func Execute() []error {
	var errors []error
	for _, migration := range migrations {
		log.Infof("Executing migration %s", migration.Name)
		err := migration.Func()
		if err != nil {
			log.Errorf("Migration %s failed: %v", migration.Name, err)
			errors = append(errors, err)
		}
	}

	return errors
}
