package routes_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	// . "github.com/onsi/gomega"
	// "github.com/redhatinsights/edge-api/pkg/routes"
	// "github.com/redhatinsights/edge-api/pkg/services/mock_services"
)

var _ = Describe("Ownershipvoucher", func() {
	var ctrl *gomock.Controller
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})
	AfterEach(func() {
		ctrl.Finish()
	})
})
