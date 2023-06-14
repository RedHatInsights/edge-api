package main_test

import (
	"os"

	. "github.com/onsi/ginkgo"
)

var _ = Describe("Delete orphaned images", func() {
	It("Won't do anything if the feature is disabled", func() {
		os.Setenv("FEATURE_DELETE_ORPHANED_IMAGES", "0")
	})
})
