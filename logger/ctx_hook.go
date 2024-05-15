package logger

import (
	"github.com/redhatinsights/edge-api/pkg/jobs"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
	"github.com/sirupsen/logrus"
)

// Use request id from the standard context and add it to the message as a field.
type ctxHook struct {
}

func (h *ctxHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *ctxHook) Fire(e *logrus.Entry) error {
	if e.Context == nil {
		return nil
	}

	id := identity.GetIdentity(e.Context)
	if id.Identity.OrgID != "" {
		e.Data["org_id"] = id.Identity.OrgID
	}

	if id.Identity.AccountNumber != "" {
		e.Data["account_number"] = id.Identity.AccountNumber
	}

	if id.Identity.User != nil && id.Identity.User.UserID != "" {
		e.Data["user_id"] = id.Identity.User.UserID
	}

	jid := jobs.JobID(e.Context)
	if jid != "" {
		e.Data["job_id"] = jid
	}

	cid := jobs.CorrID(e.Context)
	if cid != "" {
		e.Data["correlation_id"] = cid
	}

	rid := request_id.GetReqID(e.Context)
	if rid != "" {
		e.Data["request_id"] = rid
	}

	return nil
}
