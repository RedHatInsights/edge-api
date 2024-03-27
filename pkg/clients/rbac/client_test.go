// FIXME: golangci-lint
// nolint:revive
package rbac_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/internal/testing"
	"github.com/redhatinsights/edge-api/pkg/clients/rbac"
	log "github.com/sirupsen/logrus"
)

func StringPointer(str string) *string {
	return &str
}

var _ = Describe("Rbac Client", func() {
	var ctx context.Context
	var client rbac.ClientInterface

	var initialAuth bool
	var initialRbacBaseURL string

	BeforeEach(func() {
		initialAuth = config.Get().Auth
		initialRbacBaseURL = config.Get().RbacBaseURL

		config.Get().Auth = true

		ctx = context.Background()
		ctx = testing.WithRawIdentity(ctx, faker.UUIDHyphenated())
		client = rbac.InitClient(ctx, log.NewEntry(log.StandardLogger()))
	})

	AfterEach(func() {
		config.Get().Auth = initialAuth
		config.Get().RbacBaseURL = initialRbacBaseURL
		rbac.HTTPGetCommand = http.MethodGet
		rbac.IOReadAll = io.ReadAll
	})

	Context("GetAccessList", func() {

		It("should return rbac access list successfully", func() {
			groupUUID1 := faker.UUIDHyphenated()
			groupUUID2 := faker.UUIDHyphenated()
			expectedACL := rbac.AccessList{rbac.Access{
				ResourceDefinitions: []rbac.ResourceDefinition{
					{
						Filter: rbac.ResourceDefinitionFilter{
							Key:       "group.id",
							Operation: "in",
							Value:     []*string{&groupUUID1, &groupUUID2, nil},
						},
					},
				},
				Permission: "inventory:hosts:read",
			}}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				Expect(r.URL.Path).To(Equal(rbac.APIPath + "/access/"))
				Expect(r.URL.Query().Get("application")).To(Equal(string(rbac.ApplicationInventory)))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := rbac.ResponseBody{Data: expectedACL}
				err := json.NewEncoder(w).Encode(&response)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()

			config.Get().RbacBaseURL = ts.URL

			acl, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).ToNot(HaveOccurred())
			Expect(len(acl) > 0).To(BeTrue())
			Expect(len(acl)).To(Equal(len(expectedACL)))
			Expect(acl[0].Permission).To(Equal(expectedACL[0].Permission))
			Expect(len(acl[0].ResourceDefinitions)).To(Equal(len(expectedACL[0].ResourceDefinitions)))
			Expect(acl[0].ResourceDefinitions[0].Filter.Key).To(Equal(expectedACL[0].ResourceDefinitions[0].Filter.Key))
			Expect(acl[0].ResourceDefinitions[0].Filter.Operation).To(Equal(expectedACL[0].ResourceDefinitions[0].Filter.Operation))
			Expect(len(acl[0].ResourceDefinitions[0].Filter.Value)).To(Equal(len(expectedACL[0].ResourceDefinitions[0].Filter.Value)))
			for ind := range acl[0].ResourceDefinitions[0].Filter.Value {
				if acl[0].ResourceDefinitions[0].Filter.Value[ind] == nil {
					Expect(acl[0].ResourceDefinitions[0].Filter.Value[ind]).To(Equal(expectedACL[0].ResourceDefinitions[0].Filter.Value[ind]))
				} else {
					Expect(*acl[0].ResourceDefinitions[0].Filter.Value[ind]).To(Equal(*expectedACL[0].ResourceDefinitions[0].Filter.Value[ind]))
				}
			}
		})

		It("should return error when rbac url is not valid", func() {
			config.Get().RbacBaseURL = "\t"
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(MatchError(rbac.ErrCreatingRbacURL))
		})

		It("should return error when GetAccess fail", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer ts.Close()
			config.Get().RbacBaseURL = ts.URL

			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(rbac.ErrRbacRequestResponse.Error()))
		})

		It("should return error when NewRequestWithContext fails", func() {
			rbac.HTTPGetCommand = "\tBAD-HTTP-METHOD"
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(rbac.ErrFailedToBuildAccessRequest))
		})

		It("should return error when client.Do fails", func() {
			config.Get().RbacBaseURL = "url-without-schema"
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported protocol scheme"))
		})

		It("should return error when body read all fails", func() {
			expectedACL := rbac.AccessList{rbac.Access{}}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := rbac.ResponseBody{Data: expectedACL}
				err := json.NewEncoder(w).Encode(&response)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()

			config.Get().RbacBaseURL = ts.URL

			expectedError := errors.New("expected error for when reading response body fails")
			rbac.IOReadAll = func(r io.Reader) ([]byte, error) {
				return nil, expectedError
			}
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(expectedError))
		})

		It("should return error when unmarshal fails", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := `{"data: {}}`
				err := json.NewEncoder(w).Encode(&response)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()

			config.Get().RbacBaseURL = ts.URL

			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cannot unmarshal string into Go value of type rbac.ResponseBody"))

		})
	})

	Context("GetInventoryGroupsAccess", func() {

		It("it should return the expected data successfully", func() {
			groupUUID1 := faker.UUIDHyphenated()
			groupUUID2 := faker.UUIDHyphenated()
			groupUUID3 := faker.UUIDHyphenated()
			groupUUID4 := faker.UUIDHyphenated()
			expectedGroups := []string{groupUUID1, groupUUID2, groupUUID3, groupUUID4}
			// not in access list
			groupUUID5 := faker.UUIDHyphenated()
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&groupUUID1, &groupUUID2},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&groupUUID3},
							},
						},
					},
					Permission: "inventory:hosts:*",
				},
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&groupUUID4},
							},
						},
					},
					Permission: "inventory:*:read",
				},
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:*:*",
				},
				// should not be taken into account
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{&groupUUID5},
							},
						},
					},
					Permission: "inventory:groups:read",
				},
			}
			allowedAccess, overallGroupIDS, hostsWithNoGroupsAssigned, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeTrue())
			Expect(hostsWithNoGroupsAssigned).To(BeTrue())
			Expect(overallGroupIDS).To(Equal(expectedGroups))
		})

		It("it should allowed access when resources is empty", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{},
					Permission:          "inventory:hosts:read",
				},
			}
			allowedAccess, overallGroupIDS, hostsWithNoGroupsAssigned, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeTrue())
			Expect(hostsWithNoGroupsAssigned).To(BeFalse())
			Expect(len(overallGroupIDS)).To(Equal(0))
		})

		It("it should allowed access when resources is empty any where in the list 1", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{},
					Permission:          "inventory:hosts:read",
				},
			}
			allowedAccess, overallGroupIDS, hostsWithNoGroupsAssigned, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeTrue())
			Expect(hostsWithNoGroupsAssigned).To(BeFalse())
			Expect(len(overallGroupIDS)).To(Equal(0))
		})

		It("it should allowed access when resources is empty any where in the list 2", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{},
					Permission:          "inventory:hosts:read",
				},
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			allowedAccess, overallGroupIDS, hostsWithNoGroupsAssigned, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeTrue())
			Expect(hostsWithNoGroupsAssigned).To(BeFalse())
			Expect(len(overallGroupIDS)).To(Equal(0))
		})

		It("it should not be allowed access when no resources matches", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:groups:read",
				},
			}
			allowedAccess, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeFalse())
		})

		It("it should not be allowed access when no access type matches", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{nil},
							},
						},
					},
					Permission: "inventory:hosts:write",
				},
			}
			allowedAccess, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeFalse())
		})

		It("hostsWithNoGroupsAssigned should be false when no null value found", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			allowedAccess, _, hostsWithNoGroupsAssigned, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeTrue())
			Expect(hostsWithNoGroupsAssigned).To(BeFalse())
		})

		It("should return error when filter key is not valid", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "invalid.key",
								Operation: "in",
								Value:     []*string{},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			_, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(rbac.ErrInvalidAttributeFilterKey))
		})

		It("should return error when filter operation is not valid", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "invalid-operation",
								Value:     []*string{},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			_, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(rbac.ErrInvalidAttributeFilterOperation))
		})

		It("should return error when filter value item is not a valid uuid", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{StringPointer("invalid-uuid-value")},
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			_, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(rbac.ErrInvalidAttributeFilterValue))
		})

		It("should not allow access when permission is mal formed", func() {
			acl := rbac.AccessList{
				rbac.Access{
					ResourceDefinitions: []rbac.ResourceDefinition{
						{
							Filter: rbac.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     []*string{},
							},
						},
					},
					Permission: "inventory-hosts-read",
				},
			}
			allowedAccess, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).ToNot(HaveOccurred())
			Expect(allowedAccess).To(BeFalse())
		})
	})
})
