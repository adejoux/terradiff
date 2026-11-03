// Command terradiff compares two versions of a Terraform provider schema, as
// produced by `terraform providers schema -json`, and writes human-readable
// release notes in Markdown.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/adejoux/terradiff/internal/diff"
	"github.com/adejoux/terradiff/internal/render"
	"github.com/adejoux/terradiff/internal/schema"
)

// version is set at build time with -ldflags "-X main.version=...".
var version = ""

const usage = `terradiff compares two Terraform provider schemas and writes release notes.

Usage:
  terradiff [flags] OLD.json NEW.json

Generate the inputs with:
  terraform providers schema -json > schema.json

Use "-" for one of the files to read it from stdin.

Flags:
`

// exitBreaking is the exit code used by --fail-on-breaking.
const exitBreaking = 2

func main() {
	code, err := run(os.Args[1:], os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "terradiff:", err)
	}
	os.Exit(code)
}

func run(args []string, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("terradiff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprint(stderr, usage)
		fs.PrintDefaults()
	}

	var (
		output         = fs.String("o", "", "write the output to `file` instead of stdout")
		format         = fs.String("format", "markdown", "output `format`: markdown or json")
		oldVersion     = fs.String("old-version", "", "label of the old provider `version`, shown in the title (e.g. v1.2.0)")
		newVersion     = fs.String("new-version", "", "label of the new provider `version`, shown in the title (e.g. v1.3.0)")
		title          = fs.String("title", "", "custom document `title`")
		headingLevel   = fs.Int("heading-level", 1, "`level` of the top Markdown heading (use 2+ to embed in a CHANGELOG)")
		noTitle        = fs.Bool("no-title", false, "omit the top heading")
		noSummary      = fs.Bool("no-summary", false, "omit the summary table")
		descriptions   = fs.Bool("descriptions", false, "include documentation-only changes (descriptions)")
		provider       = fs.String("provider", "", "only compare this provider `address` (full address or suffix, e.g. hashicorp/aws)")
		failOnBreaking = fs.Bool("fail-on-breaking", false, fmt.Sprintf("exit with code %d when breaking changes are found", exitBreaking))
		showVersion    = fs.Bool("version", false, "print the terradiff version and exit")
	)

	positional, err := parseInterspersed(fs, args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 1, nil // the flag package already printed the error
	}
	if *showVersion {
		fmt.Fprintln(stdout, "terradiff", buildVersion())
		return 0, nil
	}
	if len(positional) != 2 {
		fs.Usage()
		return 1, errors.New("expected exactly two schema files: OLD.json NEW.json")
	}
	if positional[0] == "-" && positional[1] == "-" {
		return 1, errors.New("only one of the two inputs can be read from stdin")
	}
	if *format != "markdown" && *format != "md" && *format != "json" {
		return 1, fmt.Errorf("unknown format %q (expected markdown or json)", *format)
	}

	oldDoc, err := schema.Load(positional[0])
	if err != nil {
		return 1, fmt.Errorf("%s: %w", positional[0], err)
	}
	newDoc, err := schema.Load(positional[1])
	if err != nil {
		return 1, fmt.Errorf("%s: %w", positional[1], err)
	}

	report, err := diff.Compare(oldDoc, newDoc, diff.Options{
		Descriptions: *descriptions,
		Provider:     *provider,
	})
	if err != nil {
		return 1, err
	}

	var w io.Writer = stdout
	if *output != "" && *output != "-" {
		f, err := os.Create(*output)
		if err != nil {
			return 1, err
		}
		defer f.Close()
		w = f
	}

	switch *format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if report.Changes == nil {
			report.Changes = []diff.Change{}
		}
		err = enc.Encode(report)
	default:
		err = render.Markdown(w, report, render.Options{
			Title:        *title,
			OldVersion:   *oldVersion,
			NewVersion:   *newVersion,
			HeadingLevel: *headingLevel,
			NoTitle:      *noTitle,
			NoSummary:    *noSummary,
		})
	}
	if err != nil {
		return 1, err
	}

	if *failOnBreaking && report.HasBreaking() {
		fmt.Fprintln(stderr, "terradiff: breaking changes found")
		return exitBreaking, nil
	}
	return 0, nil
}

// parseInterspersed parses flags placed before, between or after the
// positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return positional, nil
		}
		if args[0] == "--" {
			return append(positional, args[1:]...), nil
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
}

func buildVersion() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "dev"
}
