//go:build fdo
// +build fdo

package routes_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/go-chi/chi/v5"
	. "github.com/onsi/ginkgo/v2" // nolint: revive
	. "github.com/onsi/gomega"    // nolint: revive
	"github.com/redhatinsights/edge-api/pkg/dependencies"
	"github.com/redhatinsights/edge-api/pkg/routes"

	"github.com/redhatinsights/edge-api/config"
)

var _ = Describe("Ownershipvoucher", func() {
	// fdo mock side
	config.Init()
	var fdoMockServer *http.Server
	var resRec *httptest.ResponseRecorder

	// edge-api side
	router := chi.NewRouter()
	router.Use(dependencies.Middleware)
	routes.MakeFDORouter(router)

	ovb, err := ioutil.ReadFile("/testdevice1.ov")
	fdoUUIDList := []string{"fdo-uuid-1", "fdo-uuid-2"}
	fdoUUIDListAsBytes, _ := json.Marshal(fdoUUIDList)

	_ = BeforeSuite(func() {
		// FDO mock server
		listener, _ := net.Listen("tcp", ":0")
		fdoMockServer = &http.Server{
			Addr: fmt.Sprint("http://localhost:", listener.Addr().(*net.TCPAddr).Port),
		}
		config.Get().FDO.URL = fdoMockServer.Addr // set FDO mock server address in config
		http.HandleFunc(fmt.Sprintf("/management/%s/ownership_voucher", config.Get().FDO.APIVersion), func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		})
		http.HandleFunc(fmt.Sprintf("/management/%s/ownership_voucher/delete", config.Get().FDO.APIVersion), func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
		go fdoMockServer.Serve(listener)
	})

	_ = AfterSuite(func() {
		// close FDO mock server
		fdoMockServer.Close()
	})

	Context("read ov", func() {
		It("should succeed", func() {
			Expect(err).To(BeNil())
			Expect(ovb).ToNot(BeNil())
		})
	})

	Context("routes are OK", func() {
		It("should have valid routes", func() {
			for _, route := range router.Routes() {
				for _, subRoute := range route.SubRoutes.Routes() {
					Expect(subRoute.Pattern).ToNot(BeEmpty())
					Expect(subRoute.Pattern).To(Or(Equal("/"), Equal("/connect"), Equal("/delete"), Equal("/parse")))
				}
			}
		})
	})

	Context("create ownership vouchers", func() {
		It("should succeed", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusCreated))
		})
		It("create request without body", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", nil)
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request with invalid body", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer([]byte("invalid")))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request with bad X-Number-Of-Vouchers header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "0")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request without X-Number-Of-Vouchers header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request with invalid X-Number-Of-Vouchers header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "false")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request with invalid Content-Type header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("create request with invalid Accept header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher", bytes.NewBuffer(ovb))
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("X-Number-Of-Vouchers", "1")
			req.Header.Add("Accept", "application/xml")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("delete ownership vouchers", func() {
		It("should succeed", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusOK))
		})
		It("delete request without body", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", nil)
			resRec = httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("delete request with empty body", func() {
			var s []string
			b, _ := json.Marshal(s)
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(b))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("delete request with wrong body", func() {
			s := "not a string array"
			b, _ := json.Marshal(s)
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(b))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("delete request with invalid Accept header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/xml")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("delete request with invalid Content-Type header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/xml")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
	})
	Context("parse ownership vouchers", func() {
		It("should succeed", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/parse", bytes.NewBuffer(ovb))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusOK))
		})
		It("parse request without body", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/parse", nil)
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("parse request with wrong Content-Type header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/parse", bytes.NewBuffer(ovb))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/xml")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("parse request with invalid Accept header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/parse", bytes.NewBuffer(ovb))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/cbor")
			req.Header.Add("Accept", "application/xml")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("connect devices", func() {
		It("should not succeed", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/connect", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("connect request with empty body", func() {
			var s []string
			b, _ := json.Marshal(s)
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/connect", bytes.NewBuffer(b))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusOK))
		})
		It("connect request without body", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/connect", nil)
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("connect request with wrong Content-Type header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/connect", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/xml")
			req.Header.Add("Accept", "application/json")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
		It("connect request with invalid Accept header", func() {
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/connect", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/xml")
			router.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusBadRequest))
		})
	})

	Context("bad router", func() {
		It("should fail", func() {
			badRouter := chi.NewRouter()
			badRouter.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					edgeAPIServices := &dependencies.EdgeAPIServices{}
					ctx := dependencies.ContextWithServices(r.Context(), edgeAPIServices)
					next.ServeHTTP(w, r.WithContext(ctx))
				})
			})
			routes.MakeFDORouter(badRouter)
			req, _ := http.NewRequest(http.MethodPost, "/ownership_voucher/delete", bytes.NewBuffer(fdoUUIDListAsBytes))
			resRec := httptest.NewRecorder()
			req.Header.Add("Content-Type", "application/json")
			req.Header.Add("Accept", "application/json")
			badRouter.ServeHTTP(resRec, req)
			Expect(resRec.Code).To(Equal(http.StatusInternalServerError))
		})
	})
})
