//go:build banproof

package logging

// This file logs through the process's default loggers and the built-in print functions on purpose,
// each way the ban refuses, through an aliased import too. The statements at the end log through a
// logger it was handed and set the defaults, which the ban allows, so a ban grown too wide reports a
// finding no annotation wants.
import (
	"context"
	stdlog "log"
	"log/slog"
	"os"
)

func logThroughTheDefaults(ctx context.Context) {
	logger := slog.Default()                // want forbidigo `use of .slog\.Default. forbidden because "code logs through the logger it was handed`
	slog.Debug("d")                         // want forbidigo `use of .slog\.Debug. forbidden because "code logs through the logger it was handed`
	slog.DebugContext(ctx, "d")             // want forbidigo `use of .slog\.DebugContext. forbidden because "code logs through the logger it was handed`
	slog.Info("i")                          // want forbidigo `use of .slog\.Info. forbidden because "code logs through the logger it was handed`
	slog.InfoContext(ctx, "i")              // want forbidigo `use of .slog\.InfoContext. forbidden because "code logs through the logger it was handed`
	slog.Warn("w")                          // want forbidigo `use of .slog\.Warn. forbidden because "code logs through the logger it was handed`
	slog.WarnContext(ctx, "w")              // want forbidigo `use of .slog\.WarnContext. forbidden because "code logs through the logger it was handed`
	slog.Error("e")                         // want forbidigo `use of .slog\.Error. forbidden because "code logs through the logger it was handed`
	slog.ErrorContext(ctx, "e")             // want forbidigo `use of .slog\.ErrorContext. forbidden because "code logs through the logger it was handed`
	slog.Log(ctx, slog.LevelInfo, "l")      // want forbidigo `use of .slog\.Log. forbidden because "code logs through the logger it was handed`
	slog.LogAttrs(ctx, slog.LevelInfo, "l") // want forbidigo `use of .slog\.LogAttrs. forbidden because "code logs through the logger it was handed`
	warn := slog.Warn                       // want forbidigo `use of .slog\.Warn. forbidden because "code logs through the logger it was handed`
	stdlog.Print("p")                       // want forbidigo `use of .stdlog\.Print. forbidden because "code logs through the logger it was handed`
	stdlog.Printf("p")                      // want forbidigo `use of .stdlog\.Printf. forbidden because "code logs through the logger it was handed`
	stdlog.Println("p")                     // want forbidigo `use of .stdlog\.Println. forbidden because "code logs through the logger it was handed`
	err := stdlog.Output(1, "o")            // want forbidigo `use of .stdlog\.Output. forbidden because "code logs through the logger it was handed`
	_ = stdlog.Default()                    // want forbidigo `use of .stdlog\.Default. forbidden because "code logs through the logger it was handed`
	_ = stdlog.Writer()                     // want forbidigo `use of .stdlog\.Writer. forbidden because "code logs through the logger it was handed`
	print("p")                              // want forbidigo `use of .print. forbidden because "a built-in print writes plain text to standard error`
	println("p")                            // want forbidigo `use of .println. forbidden because "a built-in print writes plain text to standard error`
	if ctx == nil {
		stdlog.Fatal("f")   // want forbidigo `use of .stdlog\.Fatal. forbidden because "code logs through the logger it was handed`
		stdlog.Fatalf("f")  // want forbidigo `use of .stdlog\.Fatalf. forbidden because "code logs through the logger it was handed`
		stdlog.Fatalln("f") // want forbidigo `use of .stdlog\.Fatalln. forbidden because "code logs through the logger it was handed`
		stdlog.Panic("p")   // want forbidigo `use of .stdlog\.Panic. forbidden because "code logs through the logger it was handed`
		stdlog.Panicf("p")  // want forbidigo `use of .stdlog\.Panicf. forbidden because "code logs through the logger it was handed`
		stdlog.Panicln("p") // want forbidigo `use of .stdlog\.Panicln. forbidden because "code logs through the logger it was handed`
	}

	handed := New(os.Stdout, slog.LevelInfo)
	handed.Info("i")
	handed.Log(ctx, slog.LevelWarn, "l")
	handed.With("k", "v").Error("e")
	slog.SetDefault(handed)
	stdlog.SetOutput(os.Stdout)
	_, _, _ = logger, warn, err
}
