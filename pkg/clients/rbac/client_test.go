package rbac_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients/rbac"
	"github.com/redhatinsights/edge-api/pkg/routes/common"
	"github.com/redhatinsights/platform-go-middlewares/identity"

	rbacClient "github.com/RedHatInsights/rbac-client-go"

	"github.com/bxcodec/faker/v3"
	. "github.com/onsi/ginkgo" // nolint: revive
	. "github.com/onsi/gomega" // nolint: revive
	log "github.com/sirupsen/logrus"
)

var _ = Describe("Rbac Client", func() {
	var orgID string
	var rhID identity.XRHID
	var ctx context.Context
	var client rbac.ClientInterface

	var initialAuth bool
	var initialRbacBaseURL string

	BeforeEach(func() {
		orgID = faker.UUIDHyphenated()
		initialAuth = config.Get().Auth
		initialRbacBaseURL = config.Get().RbacBaseURL

		config.Get().Auth = true

		rhID = identity.XRHID{Identity: identity.Identity{OrgID: orgID, Type: "User"}}
		content, err := json.Marshal(&rhID)
		Expect(err).ToNot(HaveOccurred())
		ctx = context.Background()
		ctx = common.SetOriginalIdentity(ctx, string(content))
		client = rbac.InitClient(ctx, log.NewEntry(log.StandardLogger()))
	})

	AfterEach(func() {
		config.Get().Auth = initialAuth
		config.Get().RbacBaseURL = initialRbacBaseURL
	})

	Context("GetAccessList", func() {

		It("should return rbac access list successfully", func() {
			expectedACL := rbacClient.AccessList{rbacClient.Access{}}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				response := rbacClient.PaginatedBody{Data: expectedACL}
				err := json.NewEncoder(w).Encode(&response)
				Expect(err).ToNot(HaveOccurred())
			}))
			defer ts.Close()

			config.Get().RbacBaseURL = ts.URL

			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return error when rbac url is not valid", func() {
			config.Get().RbacBaseURL = "\t"
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(MatchError(rbac.ErrCreatingRbacURL))
		})

		It("should return error when unble to get identity from context", func() {
			client = rbac.InitClient(context.Background(), log.NewEntry(log.StandardLogger()))
			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(MatchError(rbac.ErrGettingIdentityFromContext))
		})

		It("should return error when GetAccess fail", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer ts.Close()
			config.Get().RbacBaseURL = ts.URL

			_, err := client.GetAccessList(rbac.ApplicationInventory)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(" received non-OK status code: 500"))
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
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     fmt.Sprintf(`["%s","%s"]`, groupUUID1, groupUUID2)},
						},
					},
					Permission: "inventory:hosts:read",
				},
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     fmt.Sprintf(`["%s"]`, groupUUID3),
							},
						},
					},
					Permission: "inventory:hosts:*",
				},
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     fmt.Sprintf(`["%s"]`, groupUUID4)},
						},
					},
					Permission: "inventory:*:read",
				},
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     `[null]`,
							},
						},
					},
					Permission: "inventory:*:*",
				},
				// should not be taken into account
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     fmt.Sprintf(`["%s"]`, groupUUID5)},
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

		It("it should not be allowed access when no resources matches", func() {
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     `[null]`,
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
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     `[null]`,
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
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     `[]`,
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
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "invalid.key",
								Operation: "in",
								Value:     `[]`,
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
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "invalid-operation",
								Value:     `[]`,
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

		It("should return error when filter value has invalid type", func() {
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								// set a dict instead of list
								Value: `{"id": "some-value"}`,
							},
						},
					},
					Permission: "inventory:hosts:read",
				},
			}
			_, _, _, err := client.GetInventoryGroupsAccess(acl, rbac.ResourceTypeHOSTS, rbac.AccessTypeRead)
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(rbac.ErrInvalidAttributeFilterValueType))
		})

		It("should return error when filter value item is not a valid uuid", func() {
			acl := rbacClient.AccessList{
				rbacClient.Access{
					ResourceDefinitions: []rbacClient.ResourceDefinition{
						{
							Filter: rbacClient.ResourceDefinitionFilter{
								Key:       "group.id",
								Operation: "in",
								Value:     `["invalid-uuid-value"]`,
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
	})
})
