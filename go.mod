module github.com/redhatinsights/edge-api

require (
	github.com/Unleash/unleash-client-go/v4 v4.1.1
	github.com/aws/aws-sdk-go v1.54.18
	github.com/bxcodec/faker/v3 v3.8.1
	github.com/cavaliercoder/grab v2.0.0+incompatible
	github.com/confluentinc/confluent-kafka-go v1.9.2
	github.com/fedora-iot/fido-device-onboard-rs/libfdo-data-go v0.0.0-20211210154920-5c241beb5c4e
	github.com/getkin/kin-openapi v0.126.0
	github.com/getsentry/sentry-go v0.28.1
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-openapi/runtime v0.27.1
	github.com/golang/mock v1.6.0
	github.com/google/uuid v1.6.0
	github.com/knqyf263/go-rpm-version v0.0.0-20170716094938-74609b86c936
	github.com/lib/pq v1.10.9
	github.com/magiconair/properties v1.8.7
	github.com/oapi-codegen/runtime v1.1.1
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.33.1
	github.com/prometheus/client_golang v1.19.1
	github.com/redhatinsights/app-common-go v1.6.8
	github.com/redhatinsights/platform-go-middlewares/v2 v2.0.0-beta.2
	github.com/sirupsen/logrus v1.9.3
	github.com/spf13/viper v1.18.1
	github.com/stretchr/testify v1.9.0
	go.openly.dev/pointy v1.3.0
	gorm.io/driver/postgres v1.5.4
	gorm.io/driver/sqlite v1.5.4
	gorm.io/gorm v1.25.6
)

require (
	github.com/Masterminds/semver/v3 v3.1.1 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-openapi/analysis v0.21.5 // indirect
	github.com/go-openapi/errors v0.21.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.3 // indirect
	github.com/go-openapi/loads v0.21.3 // indirect
	github.com/go-openapi/spec v0.20.12 // indirect
	github.com/go-openapi/strfmt v0.21.10 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.22.4 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/invopop/yaml v0.3.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20221227161230-091c0ba34f0a // indirect
	github.com/jackc/pgx/v5 v5.4.3 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-sqlite3 v1.14.17 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/perimeterx/marshmallow v1.1.5 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.48.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	go.mongodb.org/mongo-driver v1.13.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.22.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/net v0.24.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

// fixes CVE-2023-44487
replace google.golang.org/grpc => google.golang.org/grpc v1.58.3

go 1.20
