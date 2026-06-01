// Package output renders command results in the selected format
// (table | json | yaml). Tables use text/tabwriter to avoid pulling a
// third-party dep for what is effectively spreadsheet rendering.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	sigsyaml "sigs.k8s.io/yaml"
)

const (
	FormatTable = "table"
	FormatJSON  = "json"
	FormatYAML  = "yaml"
)

// Render dispatches on format. For "table", the caller must additionally
// pass headers + a per-row callback via Table(); Render() handles only the
// machine-readable formats.
func Render(w io.Writer, format string, v any) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(v)
	case FormatYAML:
		// sigs.k8s.io/yaml marshals via JSON tags, so snake_case keys from
		// the API types render correctly without duplicating tags.
		out, err := sigsyaml.Marshal(v)
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	case FormatTable:
		return fmt.Errorf("output: Render() does not handle table format; call Table() instead")
	default:
		return fmt.Errorf("output: unknown format %q (want table|json|yaml)", format)
	}
}

// Table writes a tab-aligned table. headers is the column header row;
// rows is an N×len(headers) slice. Each cell is rendered with Sprint, so
// any string-able value works. Use this directly when format == "table".
func Table(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, strings.Join(headers, "\t")); err != nil {
		return err
	}
	for _, row := range rows {
		if _, err := fmt.Fprintln(tw, strings.Join(row, "\t")); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// Truncate shortens s to n runes (3 trailing as "..."). For monitor URLs
// and similar columns where readability beats completeness in table view.
func Truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 3 {
		return string(r[:n])
	}
	return string(r[:n-3]) + "..."
}
