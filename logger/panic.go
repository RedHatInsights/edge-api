package logger

import "log/slog"

func LogErrorAndPanic(str string, err error) {
	slog.Error(str, slog.String("error", err.Error()))
	panic(str + ": " + err.Error())
}
