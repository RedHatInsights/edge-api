package fdo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/redhatinsights/edge-api/config"
	"github.com/redhatinsights/edge-api/pkg/clients"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"strconv"
)

// ClientInterface is an Interface to make request to FDO onboarding server
type ClientInterface interface {
	BatchUpload(ovs []byte, numOfOVs uint) (interface{}, error)
	BatchDelete(fdoUUIDList []string) (interface{}, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx context.Context
	log *log.Entry
}

// InitClient initializes the client for FDO
func InitClient(ctx context.Context, log *log.Entry) *Client {
	return &Client{ctx: ctx, log: log}
}

var httpClient = &http.Client{}

// Decode response body into json
func decodeResBody(body *io.ReadCloser) (interface{}, error) {
	var j interface{}
	err := json.NewDecoder(*body).Decode(&j)
	return j, err
}

// BatchUpload used to batch upload ownershipvouchers, CBOR is self-describing, so it is possible
// to determine the end of the ownership voucher from its content
func (c *Client) BatchUpload(ovs []byte, numOfOVs uint) (interface{}, error) {
	if len(ovs) == 0 || numOfOVs == 0 {
		// c.log.Error("No ownership vouchers provided")
		return nil, errors.New("no ownership vouchers provided")
	}
	var (
		host                = config.Get().FDO.URL
		version             = config.Get().FDO.APIVersion
		authorizationBearer = fmt.Sprint("Bearer ", config.Get().FDO.AuthorizationBearer)
	)
	c.log.Info("FDO batch upload")
	// build request
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/management/%s/ownership_voucher", host, version), bytes.NewReader(ovs))
	req.Header.Add("Content-Type", "application/cbor")
	req.Header.Add("Authorization", authorizationBearer)
	req.Header.Add("X-Number-Of-Vouchers", strconv.FormatUint(uint64(numOfOVs), 10))
	req.Header.Add("Accept", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	res, err := httpClient.Do(req)
	// handle response
	if err != nil {
		c.log.Error("Failed to perform api call to upload vouchers ", err)
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusCreated:
		c.log.Info("Ownershipvouchers got created successfully")
	case http.StatusBadRequest:
		c.log.Error("Ownershipvouchers couldn't be created, bad request")
	default:
		c.log.Error("Ownershipvouchers couldn't be created, unknown error with status code: ", res.StatusCode)
	}
	return decodeResBody(&res.Body)
}

// BatchDelete used to batch delete ownershipvouchers, received as an array of
// FDO GUIDs, e.g.: [“a9bcd683-a7e4-46ed-80b2-6e55e8610d04”, “1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad”]
func (c *Client) BatchDelete(fdoUUIDList []string) (interface{}, error) {
	if len(fdoUUIDList) == 0 {
		c.log.Error("No FDO UUIDs provided")
		return nil, errors.New("no FDO UUIDs provided")
	}
	var (
		host                = config.Get().FDO.URL
		version             = config.Get().FDO.APIVersion
		authorizationBearer = fmt.Sprint("Bearer ", config.Get().FDO.AuthorizationBearer)
	)
	c.log.Info("FDO batch delete")
	fdoUUIDListAsBytes, err := json.Marshal(fdoUUIDList)
	if err != nil {
		c.log.Error("Couldn't marshal FDO GUIDs into json")
		return nil, err
	}
	// build request
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/management/%s/ownership_voucher/delete", host, version), bytes.NewReader(fdoUUIDListAsBytes))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", authorizationBearer)
	req.Header.Add("Accept", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}

	res, err := httpClient.Do(req)
	// handle response
	if err != nil {
		c.log.Error("Failed to perform api call to remove vouchers ", err)
		return nil, err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		c.log.Info("Ownershipvouchers got removed successfully")
	case http.StatusBadRequest:
		c.log.Error("Ownershipvouchers couldn't be removed, bad request")
	default:
		c.log.Error("Ownershipvouchers couldn't be removed, unknown error with status code: ", res.StatusCode)
	}
	return decodeResBody(&res.Body)
}
