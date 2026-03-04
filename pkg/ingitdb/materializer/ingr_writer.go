package materializer

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
)

// formatINGR serializes records in INGR format.
// The first line is a metadata header: "# INGR.io | {viewName}: $ID, col2, col3, ..."
// where the record key is the "$ID" column. Subsequent lines are N lines per record
// (one JSON-encoded field value per line). N equals len(headers).
// If opts.RecordsDelimiter is true, a "#-" line is written after each record.
// The footer always starts with "# {N} records\n" (the record count line, with newline).
// If opts.IncludeHash is true, a second footer line "# sha256:{hex}" is appended (no trailing newline);
// otherwise the count line itself is the last line, also without a trailing newline.
func formatINGR(viewName string, opts ExportOptions, headers []string, records []ingitdb.IRecordEntry) ([]byte, error) {
	var buf bytes.Buffer
	// Write metadata header line
	buf.WriteString("# INGR.io | ")
	buf.WriteString(viewName)
	buf.WriteString(": ")
	for i, h := range headers {
		if i > 0 {
			buf.WriteString(", ")
		}
		buf.WriteString(h)
		if opts.ColumnTypes != nil {
			if ct, ok := opts.ColumnTypes[h]; ok {
				buf.WriteByte(':')
				buf.WriteString(string(ct))
			}
		}
	}
	buf.WriteByte('\n')
	for _, rec := range records {
		d := rec.GetData()
		for _, h := range headers {
			var val any
			if d != nil {
				val = d[h]
			}
			b, err := json.Marshal(val)
			if err != nil {
				return nil, fmt.Errorf("ingr: failed to marshal field %q: %w", h, err)
			}
			buf.Write(b)
			buf.WriteByte('\n')
		}
		if opts.RecordsDelimiter {
			buf.WriteString("#-\n")
		}
	}
	// Write record count line — always with trailing newline
	n := len(records)
	if n == 1 {
		buf.WriteString("# 1 record\n")
	} else {
		fmt.Fprintf(&buf, "# %d records\n", n)
	}
	if opts.IncludeHash {
		// Compute sha256 of all content so far (header + records + count line with \n)
		sum := sha256.Sum256(buf.Bytes())
		fmt.Fprintf(&buf, "# sha256:%x", sum)
	}
	return buf.Bytes(), nil
}
