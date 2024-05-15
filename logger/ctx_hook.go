package logger

import (
	"runtime/debug"

	"github.com/redhatinsights/edge-api/pkg/jobs"
	"github.com/redhatinsights/platform-go-middlewares/v2/identity"
	"github.com/redhatinsights/platform-go-middlewares/v2/request_id"
	"github.com/sirupsen/logrus"
)

var (
	// BuildCommit is git SHA commit
	BuildCommit string

	// BuildTime is date and time
	BuildTime string
)

func init() {
	bi, ok := debug.ReadBuildInfo()

	if !ok {
		BuildTime = "N/A"
		BuildCommit = "HEAD"
	}

	for _, bs := range bi.Settings {
		switch bs.Key {
		case "vcs.revision":
			if len(bs.Value) > 5 {
				BuildCommit = bs.Value[0:5]
			}
		case "vcs.time":
			BuildTime = bs.Value
		}
	}
}

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

// Fire sets key fields from context, when present. In some cases, multiple
// names are used across the codebase. In that case, all are set for better
// user experience in Kibana UI.
func (h *ctxHook) Fire(e *logrus.Entry) error {
	e.Data["build_commit"] = BuildCommit
	e.Data["build_time"] = BuildTime

	if e.Context == nil {
		return nil
	}

	id := identity.GetIdentity(e.Context)
	if id.Identity.OrgID != "" {
		e.Data["org_id"] = id.Identity.OrgID
		e.Data["OrgID"] = id.Identity.OrgID
	}

	if id.Identity.AccountNumber != "" {
		e.Data["account_number"] = id.Identity.AccountNumber
		e.Data["accountId"] = id.Identity.AccountNumber
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
		e.Data["requestId"] = rid
	}

	return nil
}
