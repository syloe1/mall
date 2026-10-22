package logx

import (
	"log/slog"
	"os"
)

func Init() {
	//输出到控制台， 格式是JSON, 只输出Info及以上级别
	//Debug < Info < Warn < Error
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	//设置为全局默认日志
	slog.SetDefault(slog.New(handler))
}
