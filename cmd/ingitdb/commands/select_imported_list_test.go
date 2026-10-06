package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dal-go/dalgo/dal"
	"github.com/ingitdb/dalgo2ingitdb4local"
	"github.com/ingitdb/ingitdb-go/ingitdb"
)

func TestSelectImportedListFormatsPreserveIdentityAndTypes(t *testing.T) {
	t.Parallel()
	for _, format := range []ingitdb.RecordFormat{ingitdb.RecordFormatJSONL, ingitdb.RecordFormatCSV} {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			file := "records.jsonl"
			if format == ingitdb.RecordFormatCSV {
				file = "records.csv"
			}
			col := &ingitdb.CollectionDef{
				ID: "sample", DirPath: dir,
				RecordFile: &ingitdb.RecordFileDef{Name: file, Format: format, RecordType: ingitdb.ListOfRecords},
				Columns: map[string]*ingitdb.ColumnDef{
					"part": {Type: ingitdb.ColumnTypeString}, "seq": {Type: ingitdb.ColumnTypeInt},
					"wide": {Type: ingitdb.ColumnTypeInt}, "blob": {Type: ingitdb.ColumnTypeString},
					"amount": {Type: ingitdb.ColumnTypeString},
				},
				ColumnsOrder: []string{"part", "seq", "wide", "blob", "amount"},
				PrimaryKey:   []string{"part", "seq"},
				SourceSchema: &ingitdb.SourceSchemaDef{KeyMode: "source-primary-key", Fields: []ingitdb.SourceFieldDef{
					{Name: "part", Type: "string"}, {Name: "seq", Type: "int64"},
					{Name: "wide", Type: "int64"}, {Name: "blob", Type: "bytes"},
					{Name: "amount", Type: "decimal"},
				}},
			}
			if format == ingitdb.RecordFormatCSV {
				col.RecordFile.CSVCellEncoding = "json-v1"
				col.ColumnsOrder = append([]string{"$ID"}, col.ColumnsOrder...)
			}
			rows := []map[string]any{
				{"$ID": "pk-composite", "part": "A", "seq": int64(7), "wide": int64(9007199254740993), "blob": "AAEC", "amount": "9223372036854775809"},
				{"$ID": "pk-max", "part": "B", "seq": int64(8), "wide": int64(9223372036854775807), "blob": "AAEC", "amount": "9223372036854775810"},
				{"$ID": "pk-min", "part": "C", "seq": int64(9), "wide": int64(-9223372036854775808), "blob": "AAEC", "amount": "0.1234567890123456"},
			}
			var content []byte
			var err error
			if format == ingitdb.RecordFormatCSV {
				content, err = ingitdb.EncodeRecordContentForCollection(rows, col)
			} else {
				content, err = ingitdb.EncodeListOfRecordsContent(rows, format, col.ColumnsOrder)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, file), content, 0o644); err != nil {
				t.Fatal(err)
			}
			def := &ingitdb.Definition{Collections: map[string]*ingitdb.CollectionDef{"sample": col}}
			homeDir := func() (string, error) { return dir, nil }
			getWd := func() (string, error) { return dir, nil }
			readDef := func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error) { return def, nil }
			newDB := func(root string, d *ingitdb.Definition) (dal.DB, error) {
				return dalgo2fsingitdb.NewLocalDBWithDef(root, d)
			}
			for _, args := range [][]string{
				{"--path=" + dir, "--from=sample", "--where=seq==7", "--format=json"},
				{"--path=" + dir, "--from=sample", "--where=wide==9007199254740993", "--format=json"},
				{"--path=" + dir, "--id=sample/pk-composite", "--format=json"},
			} {
				out, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {}, args...)
				if err != nil {
					t.Fatalf("select %v: %v", args, err)
				}
				for _, want := range []string{`"$id": "pk-composite"`, `"wide": 9007199254740993`, `"blob": "AAEC"`} {
					if !strings.Contains(out, want) {
						t.Errorf("select %v missing %s: %s", args, want, out)
					}
				}
			}
			out, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
				"--path="+dir, "--from=sample", "--where=wide==9007199254740992", "--format=json")
			if err != nil || strings.Contains(out, "pk-composite") {
				t.Fatalf("adjacent wide integer matched: %q, %v", out, err)
			}
			out, err = runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
				"--path="+dir, "--from=sample", `--where=blob=="AAEC"`, "--format=json")
			if err != nil || strings.Contains(out, "pk-composite") {
				t.Fatalf("binary/string comparison = %q, %v", out, err)
			}
			out, err = runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
				"--path="+dir, "--from=sample", `--where=blob=="[0 1 2]"`, "--format=json")
			if err != nil || strings.Contains(out, "pk-composite") {
				t.Fatalf("binary/text representation matched: %q, %v", out, err)
			}
			_, err = runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
				"--path="+dir, "--from=sample", `--where=blob>"AAEC"`, "--format=json")
			if err == nil || !strings.Contains(err.Error(), "ordered comparison of binary values") {
				t.Fatalf("ordered binary comparison error = %v", err)
			}
			for _, tc := range []struct{ where, id, wide string }{
				{"wide==9223372036854775807", "pk-max", "9223372036854775807"},
				{"wide==-9223372036854775808", "pk-min", "-9223372036854775808"},
				{"wide>9223372036854775806", "pk-max", "9223372036854775807"},
				{"wide<-9223372036854775807", "pk-min", "-9223372036854775808"},
			} {
				out, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
					"--path="+dir, "--from=sample", "--where="+tc.where, "--format=json")
				if err != nil || !strings.Contains(out, tc.id) || !strings.Contains(out, tc.wide) || strings.Contains(out, "pk-composite") {
					t.Fatalf("wide filter %q = %q, %v", tc.where, out, err)
				}
			}
			for _, tc := range []struct{ where, id string }{
				{"amount==9223372036854775809", "pk-composite"},
				{"amount==9223372036854775810", "pk-max"},
				{"amount>9223372036854775809", "pk-max"},
				{"amount==0.1234567890123456", "pk-min"},
			} {
				out, err := runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
					"--path="+dir, "--from=sample", "--where="+tc.where, "--format=json")
				if err != nil || !strings.Contains(out, tc.id) {
					t.Fatalf("decimal filter %q = %q, %v", tc.where, out, err)
				}
				for _, other := range []string{"pk-composite", "pk-max", "pk-min"} {
					if other != tc.id && strings.Contains(out, other) {
						t.Fatalf("decimal filter %q matched adjacent %s: %q", tc.where, other, out)
					}
				}
			}
			out, err = runSelectCmd(t, homeDir, getWd, readDef, newDB, func(...any) {},
				"--path="+dir, "--from=sample", "--order-by=amount", "--format=json")
			if err != nil {
				t.Fatalf("decimal order: %v", err)
			}
			minPos := strings.Index(out, "pk-min")
			firstPos := strings.Index(out, "pk-composite")
			maxPos := strings.Index(out, "pk-max")
			if minPos < 0 || minPos >= firstPos || firstPos >= maxPos {
				t.Fatalf("decimal order rounded or lexical: %q", out)
			}
		})
	}
}
