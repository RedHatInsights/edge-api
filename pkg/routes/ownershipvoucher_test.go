package routes_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/redhatinsights/edge-api/pkg/routes"
)

var _ = Describe("Ownershipvoucher", func() {
	ovb, err := ioutil.ReadFile("/testdevice1.ov")
	m := chi.NewRouter()
	m.Route("/", func(s chi.Router) {
		routes.MakeFDORouter(s)
	})

	Context("read ov", func() {
		It("should succeed", func() {
			Expect(err).To(BeNil())
			Expect(ovb).ToNot(BeNil())
		})
	})

	Context("router validation", func() {
		It("has two routes", func() {
			Expect(m.Routes()).To(HaveLen(2))
		})
	})

	Context("create ownership vouchers", func() {
		req, err := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
		It("create request without error", func() {
			Expect(err).To(BeNil())
		})
		resRec := httptest.NewRecorder()
		It("should succeed", func() {
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/json")
			m.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusCreated))
		})
	})
})
