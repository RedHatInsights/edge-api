module github.com/redhatinsights/edge-api

require (
	github.com/Unleash/unleash-client-go/v3 v3.7.0
	github.com/aws/aws-sdk-go v1.44.129
	github.com/bxcodec/faker/v3 v3.8.0
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/confluentinc/confluent-kafka-go v1.9.2
	github.com/fedora-iot/fido-device-onboard-rs/libfdo-data-go v0.0.0-20211210154920-5c241beb5c4e
	github.com/getkin/kin-openapi v0.107.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/go-openapi/runtime v0.24.2
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.3.0
	github.com/knqyf263/go-rpm-version v0.0.0-20170716094938-74609b86c936
	github.com/lib/pq v1.10.7
	github.com/magiconair/properties v1.8.6
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.24.0
	github.com/prometheus/client_golang v1.13.1
	github.com/redhatinsights/app-common-go v1.6.4
	github.com/redhatinsights/platform-go-middlewares v0.20.0
	github.com/sirupsen/logrus v1.9.0
	github.com/spf13/viper v1.14.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gorm.io/driver/postgres v1.4.5
	gorm.io/driver/sqlite v1.4.3
	gorm.io/gorm v1.24.1-0.20221019064659-5dd2bb482755
)

go 1.16
