package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"
)

// outputFormat is set by the root command's -o flag.
// Supported values: "table" (default), "json", "yaml".
var outputFormat string

// printTable writes tabular data to stdout using aligned columns.
func printTable(headers []string, rows [][]string) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for i, h := range headers {
		if i > 0 {
			fmt.Fprint(w, "\t")
		}
		fmt.Fprint(w, h)
	}
	fmt.Fprintln(w)
	for _, row := range rows {
		for i, col := range row {
			if i > 0 {
				fmt.Fprint(w, "\t")
			}
			fmt.Fprint(w, col)
		}
		fmt.Fprintln(w)
	}
	w.Flush()
}

// printJSON writes the value as pretty-printed JSON to stdout.
func printJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// printYAML writes the value as YAML to stdout.
func printYAML(v interface{}) error {
	enc := yaml.NewEncoder(os.Stdout)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(v)
}

// printOutput dispatches to JSON, YAML, or table output based on outputFormat.
// For table output it uses the provided headers and toRow function to convert
// each item in a slice to a row of strings.
func printOutput(v interface{}, headers []string, toRow func(interface{}) []string) {
	switch outputFormat {
	case "json":
		if err := printJSON(v); err != nil {
			exitError(fmt.Sprintf("failed to encode JSON: %v", err))
		}
	case "yaml":
		if err := printYAML(v); err != nil {
			exitError(fmt.Sprintf("failed to encode YAML: %v", err))
		}
	default:
		// Table output: v must be a slice represented as []interface{}.
		items, ok := v.([]interface{})
		if !ok {
			// Single item -- wrap in a slice.
			items = []interface{}{v}
		}
		var rows [][]string
		for _, item := range items {
			rows = append(rows, toRow(item))
		}
		printTable(headers, rows)
	}
}

// exitError prints an error message to stderr and exits with code 1.
func exitError(msg string) {
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	os.Exit(1)
}

// formatAge returns a human-readable duration string relative to the given
// time, such as "5s", "3m", "2h", "4d". Returns "<unknown>" for zero times.
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
