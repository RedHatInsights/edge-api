//go:build fdo
// +build fdo

package fdo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/redhatinsights/edge-api/pkg/clients"
	log "github.com/sirupsen/logrus"

	"github.com/redhatinsights/edge-api/config"
)

// ClientInterface is an Interface to make request to FDO onboarding server
type ClientInterface interface {
	BatchUpload(ovs []byte, numOfOVs uint) (interface{}, error)
	BatchDelete(fdoUUIDList []string) (interface{}, error)
}

// Client is the implementation of an ClientInterface
type Client struct {
	ctx        context.Context
	log        *log.Entry
	httpClient *http.Client
}

// InitClient initializes the client for FDO
func InitClient(ctx context.Context, log *log.Entry) *Client {
	return &Client{ctx: ctx, log: log, httpClient: &http.Client{}}
}

// BatchUpload used to batch upload ownershipvouchers, CBOR is self-describing, so it is possible
// to determine the end of the ownership voucher from its content
func (c *Client) BatchUpload(ovs []byte, numOfOVs uint) (interface{}, error) {
	if len(ovs) == 0 || numOfOVs == 0 {
		c.log.WithField("method", "fdo.BatchUpload").Error("No ownership vouchers provided")
		return nil, errors.New("no ownership vouchers provided")
	}
	req, _ := reqUploadBuilder(c, ovs, numOfOVs) // build request
	res, err := c.httpClient.Do(req)
	// handle response
	if err != nil {
		c.log.WithFields(log.Fields{"method": "fdo.BatchUpload", "error": err}).Error("Failed to perform api call to upload vouchers")
		return nil, err
	}
	return resUploadHandler(c, res)
}

// BatchDelete used to batch delete ownershipvouchers, received as an array of
// FDO GUIDs, e.g.: [“a9bcd683-a7e4-46ed-80b2-6e55e8610d04”, “1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad”]
func (c *Client) BatchDelete(fdoUUIDList []string) (interface{}, error) {
	if len(fdoUUIDList) == 0 {
		c.log.WithField("method", "fdo.BatchDelete").Error("No FDO UUIDs provided")
		return nil, errors.New("no FDO UUIDs provided")
	}
	req, _ := reqDeleteBuilder(c, fdoUUIDList) // build request
	res, err := c.httpClient.Do(req)
	// handle response
	if err != nil {
		c.log.WithFields(log.Fields{"method": "fdo.BatchDelete", "error": err}).Error("Failed to perform api call to remove vouchers")
		return nil, err
	}
	return resDeleteHandler(c, res)
}

// Decode response body into json
func decodeResBody(body *io.ReadCloser) interface{} {
	var j interface{}
	json.NewDecoder(*body).Decode(&j)
	return j
}

// basic config for FDO
func basicConfig() (string, string, string) {
	host := config.Get().FDO.URL
	version := config.Get().FDO.APIVersion
	authorizationBearer := fmt.Sprint("Bearer ", config.Get().FDO.AuthorizationBearer)
	return host, version, authorizationBearer
}

// build request for uploading ownershipvouchers
func reqUploadBuilder(c *Client, ovs []byte, numOfOVs uint) (*http.Request, error) {
	host, version, authorizationBearer := basicConfig()
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/management/%s/ownership_voucher", host, version), bytes.NewReader(ovs))
	req.Header.Add("Content-Type", "application/cbor")
	req.Header.Add("Authorization", authorizationBearer)
	req.Header.Add("X-Number-Of-Vouchers", strconv.FormatUint(uint64(numOfOVs), 10))
	req.Header.Add("Accept", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	return req, err
}

// handle response for uploading ownershipvouchers
func resUploadHandler(c *Client, res *http.Response) (interface{}, error) {
	var err error
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusCreated:
		err = nil
		c.log.WithField("method", "fdo.resUploadHandler").Info("Ownershipvouchers got created successfully")
	case http.StatusBadRequest:
		err = errors.New("bad request")
		c.log.WithField("method", "fdo.resUploadHandler").Error("Ownershipvouchers couldn't be created, bad request")
	default:
		err = errors.New(fmt.Sprint("unknown error with status code: ", res.StatusCode))
		c.log.WithFields(log.Fields{"method": "fdo.resUploadHandler", "statusCode": res.StatusCode}).Error("Ownershipvouchers couldn't be created, unknown error")
	}
	return decodeResBody(&res.Body), err
}

// build request for deleting ownershipvouchers
func reqDeleteBuilder(c *Client, fdoUUIDList []string) (*http.Request, error) {
	fdoUUIDListAsBytes, err := json.Marshal(fdoUUIDList)
	if err != nil {
		c.log.WithField("method", "fdo.reqDeleteBuilder").Error("Couldn't marshal FDO GUIDs into json")
		return nil, err
	}
	host, version, authorizationBearer := basicConfig()
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/management/%s/ownership_voucher/delete", host, version), bytes.NewReader(fdoUUIDListAsBytes))
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Authorization", authorizationBearer)
	req.Header.Add("Accept", "application/json")
	headers := clients.GetOutgoingHeaders(c.ctx)
	for key, value := range headers {
		req.Header.Add(key, value)
	}
	return req, err
}

// handle response for deleting ownershipvouchers
func resDeleteHandler(c *Client, res *http.Response) (interface{}, error) {
	var err error
	defer res.Body.Close()
	switch res.StatusCode {
	case http.StatusOK:
		err = nil
		c.log.WithField("method", "fdo.resDeleteHandler").Info("Ownershipvouchers got removed successfully")
	case http.StatusBadRequest:
		err = errors.New("bad request")
		c.log.WithField("method", "fdo.resDeleteHandler").Error("Ownershipvouchers couldn't be removed, bad request")
	default:
		err = errors.New(fmt.Sprint("unknown error with status code: ", res.StatusCode))
		c.log.WithFields(log.Fields{"method": "fdo.resDeleteHandler", "statusCode": res.StatusCode}).Error("Ownershipvouchers couldn't be removed, unknown error")
	}
	return decodeResBody(&res.Body), err
}
