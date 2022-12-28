// FIXME: golangci-lint
// nolint:revive
package files

import (
	"github.com/redhatinsights/edge-api/logger"

	"github.com/redhatinsights/edge-api/config"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
)

// GetNewS3Session return a new aws s3 session
func GetNewS3Session() *session.Session {
	cfg := config.Get()
	var sess *session.Session
	if cfg.Debug {
		sess = session.Must(session.NewSessionWithOptions(session.Options{
			// Force enable Shared Config support
			SharedConfigState: session.SharedConfigEnable,
		}))
	} else {
		var err error
		sess, err = session.NewSession(&aws.Config{
			Region:      &cfg.BucketRegion,
			Credentials: credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
		})
		if err != nil {
			logger.LogErrorAndPanic("failure creating new session", err)
		}
	}
	return sess
}
