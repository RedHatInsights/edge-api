# Ownership Voucher Management API

This protocol will be called the "Ownership Voucher Management API", or in the scope of this document, the "Management API".

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL
NOT", "SHOULD", "SHOULD NOT", "RECOMMENDED",  "MAY", and
"OPTIONAL" in this document are to be interpreted as described in
RFC 2119.

## General Features

### Responses

Requests that got parsed and executed succesfully will have a Status code in the Succesful range ([section 6.3, RFC 7231](https://datatracker.ietf.org/doc/html/rfc7231#section-6.3)).
If an error occured during processing, a Status code in either the Client Error ([section 6.5, RFC 7231](https://datatracker.ietf.org/doc/html/rfc7231#section-6.5)) or Server Error ([section 6.6, RFC 7231](https://datatracker.ietf.org/doc/html/rfc7231#section-6.6)) will be returned.

Responses will have `Content-Type: application/json` and consist of JSON strings.

#### Error responses

Error responses will consist of JSON objects, with at least the following keys:

- `error_code`: An operation-specific string error code.
- `error_details`: A JSON object with keys defined by the specific `error_code` value.

## Ownership Voucher upload

HTTP Request context: `POST $base/ownership_voucher`.

This endpoint can be used to upload a batch of new ownership vouchers.
A header is sent with the number of vouchers to be uploaded, so the Ownershipvoucher Service can verify that it did in fact receive (and process) every ownership voucher.
This endpoint will accept raw ownership vouchers using CBOR encoding `Content-Type application/cbor`.
The vouchers should just be appended to each other as a byte stream.

The request MUST contain a header `X-Number-Of-Vouchers`, containing the number of Ownership Vouchers being uploaded.
If this number diverges from the number of vouchers the server parsed, they should refuse the entire request.

A successful response will contain a JSON list containing objects, which each have at least the following keys:

- `guid`: the FDO GUID of the Ownership Voucher

### Error codes

- `incomplete_voucher`: when an uploaded voucher was incomplete. `error_details` contains at least the key `parsed_correctly`, containing the number of Ownership Vouchers succesfully parsed.
- `parse_error`: when an Ownership Voucher was uploaded that is structurally invalid. `error_details` contains the key `parsed_correctly`, containing the number of Ownership Vouchers succesfully parsed, and the key `description`, containing a string with a description of the parse failure.
- `invalid_number_of_vouchers`: when the value of `X-Number-Of-Vouchers` does not match the number of parsed Ownership Vouchers. `error_details` contains the key `parsed`, with an integer containing the number of Ownership Vouhcers that were encountered.
- `unowned_voucher`: when an Ownership Voucher was uploaded for which the current Owner is not the Owner key of the server. `error_details` contains the key `unowned`, which is a list of indexes of Ownership Vouchers that have invalid ownership.
- `invalid_voucher_signatures`: when an Ownership Voucher was uploaded for which one of the cryptographic verifications failed. `error_details` contains the key `invalid`, which contains a list of objects with the key `index` describing the index of the failing voucher, and `description` containing a string description of what failed to verify on the voucher.
- `invalid_header`: when the request did not contain valid headers. `error_details` contains the key `error_message`, which contains a string describing the error.
- `incomplete_body`: when the request did not contain a complete body or it is broken. `error_details` contains the key `error_message`, which contains a string describing the error.
- `validation_parse_error`: when an Ownership Voucher was failed to parse during the verification process. `error_details` contains the key `error_message`, which contains a string describing the error.


### Example

This assumes a URI base of `/fdo`.

#### Request

``` HTTP
POST /fdo/ownership_voucher HTTP/1.1
Host: edge-api.example.com
X-Number-Of-Vouchers: 3
Content-Type: application/cbor
Accept: application/json

<voucher-1-bytes><voucher-2-bytes><voucher-3-bytes>
```

#### Successful response

``` HTTP
HTTP/1.1 201 Created
Content-Type: application/json

[{"guid": "4e945116-fef1-41ab-9e75-e523476bbe14"}, {"guid": "8bff3a64-f494-4a68-8c9f-cf8d0771d9b1"}, {"guid": "f2e52413-5843-402c-96bf-9bdbc2c17bed"}]
```

#### Failed response: incomplete_voucher

``` HTTP
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error_code": "incomplete_voucher", "error_details": {"parsed_correctly": 2}}
```

#### Failed response: unowned_voucher

``` HTTP
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error_code": "unowned_voucher", "error_details": {"unowned": ["4fd43ba9-12ec-4f32-bda0-d5c0956a19be"]}}
```

## Ownership Voucher delete

HTTP Request context: `POST $base/ownership_voucher/delete`.

This endpoint can be used to request the Owner Onboarding Server to delete a set of Ownership Vouchers, and to stop taking ownerhsip of the devices.
The request body consists of a JSON list of GUIDs for which the Ownership Vouchers should get deleted.

A succesful response contains an empty body.

### Error codes

- `unknown_device`: at least one of the GUIDs that were submitted were unknown to this Owner Onboarding Service. `error_details` contains the key `unknown`, which contains a JSON list of GUIDs that were unknown to this server.
- `invalid_header`: when the request did not contain valid headers. `error_details` contains the key `error_message`, which contains a string describing the error.
- `incomplete_body`: when the request did not contain a complete body or it is broken. `error_details` contains the key `error_message`, which contains a string describing the error.

### Example

This assumes a URI base of `/fdo`.

#### Request

``` HTTP
POST /fdo/ownership_voucher/delete HTTP/1.1
Host: edge-api.example.com
Content-Type: application/json
Accept: application/json

[“a9bcd683-a7e4-46ed-80b2-6e55e8610d04”, “1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad”]
```

#### Successful response

``` HTTP
HTTP/1.1 200 OK
Content-Type: application/json
```

#### Failed response: unknown_device

``` HTTP
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error_code": "unknown_device", "error_details": {"unknown": [“1ea69fcb-b784-4d0f-ab4d-94589c6cc7ad”"]}}
```
