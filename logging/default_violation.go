//go:build banproof

package logging

// This file logs through the process's default logger and through the log package on purpose, each
// way the ban refuses, through aliased imports, a function held in a variable and the log package's
// loggers held as values. The last statements log through a logger value and set the default, which
// the ban allows, so a ban grown too wide reports a finding no annotation wants.
import (
	"context"
	"io"
	stdlog "log"
	s "log/slog"
)

func logThroughTheDefault(ctx context.Context) {
	s.Debug("detail")                    // want forbidigo `use of .s\.Debug. forbidden because "project code logs through the logger its composition root hands it`
	s.DebugContext(ctx, "detail")        // want forbidigo `use of .s\.DebugContext. forbidden because "project code logs through the logger its composition root hands it`
	s.Info("progress")                   // want forbidigo `use of .s\.Info. forbidden because "project code logs through the logger its composition root hands it`
	s.InfoContext(ctx, "progress")       // want forbidigo `use of .s\.InfoContext. forbidden because "project code logs through the logger its composition root hands it`
	s.Warn("a decision")                 // want forbidigo `use of .s\.Warn. forbidden because "project code logs through the logger its composition root hands it`
	s.WarnContext(ctx, "a decision")     // want forbidigo `use of .s\.WarnContext. forbidden because "project code logs through the logger its composition root hands it`
	s.Error("a failure")                 // want forbidigo `use of .s\.Error. forbidden because "project code logs through the logger its composition root hands it`
	s.ErrorContext(ctx, "a failure")     // want forbidigo `use of .s\.ErrorContext. forbidden because "project code logs through the logger its composition root hands it`
	s.Log(ctx, s.LevelInfo, "progress")  // want forbidigo `use of .s\.Log. forbidden because "project code logs through the logger its composition root hands it`
	s.LogAttrs(ctx, s.LevelInfo, "line") // want forbidigo `use of .s\.LogAttrs. forbidden because "project code logs through the logger its composition root hands it`
	held := s.Info                       // want forbidigo `use of .s\.Info. forbidden because "project code logs through the logger its composition root hands it`
	held("progress")
	def := s.Default() // want forbidigo `use of .s\.Default. forbidden because "project code logs through the logger its composition root hands it`

	stdlog.Printf("progress")            // want forbidigo `use of .stdlog\.Printf. forbidden because "the log package writes unstructured lines`
	stdlog.Println("progress")           // want forbidigo `use of .stdlog\.Println. forbidden because "the log package writes unstructured lines`
	std := stdlog.Default()              // want forbidigo `use of .stdlog\.Default. forbidden because "the log package writes unstructured lines`
	own := stdlog.New(io.Discard, "", 0) // want forbidigo `use of .stdlog\.New. forbidden because "the log package writes unstructured lines`
	stdlog.SetOutput(io.Discard)         // want forbidigo `use of .stdlog\.SetOutput. forbidden because "the log package writes unstructured lines`

	def.Info("progress")
	std.Print("progress") // want forbidigo `use of .std\.Print. forbidden because "the log package writes unstructured lines`
	own.Print("progress") // want forbidigo `use of .own\.Print. forbidden because "the log package writes unstructured lines`

	logger := s.New(s.NewJSONHandler(io.Discard, nil))
	logger.Info("progress")
	logger.ErrorContext(ctx, "a failure")
	s.SetDefault(logger)
}
