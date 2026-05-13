package dalgo2ghingitdb

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dal-go/dalgo/dal"
	"github.com/dal-go/dalgo/update"
	"github.com/pelletier/go-toml/v2"

	"github.com/ingitdb/ingitdb-cli/pkg/dalgo2ingitdb"
	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb"
	"gopkg.in/yaml.v3"
)

var _ dal.ReadwriteTransaction = (*readwriteTx)(nil)

type readwriteTx struct {
	readonlyTx
}

func (r readwriteTx) Set(ctx context.Context, record dal.Record) error {
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)
	record.SetError(nil)

	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		content, sha, found, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		var allRecords map[string]map[string]any
		if !found {
			allRecords = make(map[string]map[string]any)
			sha = ""
		} else {
			parseErr := error(nil)
			allRecords, parseErr = dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
			if parseErr != nil {
				return parseErr
			}
		}
		data, ok := record.Data().(map[string]any)
		if !ok {
			return fmt.Errorf("record data is not map[string]any")
		}
		allRecords[recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		encoded, encodeErr := dalgo2ingitdb.EncodeMapOfRecordsContent(
			allRecords, colDef.RecordFile.Format, colDef.ID, colDef.ColumnsOrder)
		if encodeErr != nil {
			return encodeErr
		}
		writeErr := r.db.fileReader.writeFile(ctx, recordPath, "ingitdb: set "+colDef.ID+"/"+recordKey, encoded, sha)
		if writeErr != nil {
			return writeErr
		}

	default:
		_, sha, _, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		data, ok := record.Data().(map[string]any)
		if !ok {
			return fmt.Errorf("record data is not map[string]any")
		}
		encoded, encodeErr := encodeRecordContent(data, colDef.RecordFile.Format)
		if encodeErr != nil {
			return encodeErr
		}
		writeErr := r.db.fileReader.writeFile(ctx, recordPath, "ingitdb: set "+colDef.ID+"/"+recordKey, encoded, sha)
		if writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func (r readwriteTx) Insert(ctx context.Context, record dal.Record, opts ...dal.InsertOption) error {
	_ = opts
	colDef, recordKey, err := r.resolveCollection(record.Key())
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)

	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		content, sha, found, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		var allRecords map[string]map[string]any
		if !found {
			allRecords = make(map[string]map[string]any)
			sha = ""
		} else {
			parseErr := error(nil)
			allRecords, parseErr = dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
			if parseErr != nil {
				return parseErr
			}
		}
		if _, exists := allRecords[recordKey]; exists {
			return fmt.Errorf("record already exists: %s/%s", colDef.ID, recordKey)
		}
		record.SetError(nil)
		data, ok := record.Data().(map[string]any)
		if !ok {
			return fmt.Errorf("record data is not map[string]any")
		}
		allRecords[recordKey] = dalgo2ingitdb.ApplyLocaleToWrite(data, colDef.Columns)
		encoded, encodeErr := dalgo2ingitdb.EncodeMapOfRecordsContent(
			allRecords, colDef.RecordFile.Format, colDef.ID, colDef.ColumnsOrder)
		if encodeErr != nil {
			return encodeErr
		}
		writeErr := r.db.fileReader.writeFile(ctx, recordPath, "ingitdb: insert "+colDef.ID+"/"+recordKey, encoded, sha)
		if writeErr != nil {
			return writeErr
		}

	default:
		_, _, found, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		if found {
			return fmt.Errorf("record already exists: %s/%s", colDef.ID, recordKey)
		}
		record.SetError(nil)
		data, ok := record.Data().(map[string]any)
		if !ok {
			return fmt.Errorf("record data is not map[string]any")
		}
		encoded, encodeErr := encodeRecordContent(data, colDef.RecordFile.Format)
		if encodeErr != nil {
			return encodeErr
		}
		writeErr := r.db.fileReader.writeFile(ctx, recordPath, "ingitdb: insert "+colDef.ID+"/"+recordKey, encoded, "")
		if writeErr != nil {
			return writeErr
		}
	}
	return nil
}

func (r readwriteTx) Delete(ctx context.Context, key *dal.Key) error {
	colDef, recordKey, err := r.resolveCollection(key)
	if err != nil {
		return err
	}
	recordPath := resolveRecordPath(colDef, recordKey)

	switch colDef.RecordFile.RecordType {
	case ingitdb.MapOfRecords:
		content, sha, found, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		if !found {
			return dal.ErrRecordNotFound
		}
		allRecords, parseErr := dalgo2ingitdb.ParseMapOfRecordsContent(content, colDef.RecordFile.Format)
		if parseErr != nil {
			return parseErr
		}
		if _, exists := allRecords[recordKey]; !exists {
			return dal.ErrRecordNotFound
		}
		delete(allRecords, recordKey)
		toEncode := make(map[string]any, len(allRecords))
		for k, v := range allRecords {
			toEncode[k] = v
		}
		encoded, encodeErr := encodeRecordContent(toEncode, colDef.RecordFile.Format)
		if encodeErr != nil { // untestable: ParseMapOfRecordsContent already fails for unsupported formats;
			// for supported formats (json/yaml) parsed data can always be re-encoded
			return encodeErr
		}
		writeErr := r.db.fileReader.writeFile(ctx, recordPath, "ingitdb: delete "+colDef.ID+"/"+recordKey, encoded, sha)
		if writeErr != nil {
			return writeErr
		}

	default:
		_, sha, found, readErr := r.db.fileReader.readFileWithSHA(ctx, recordPath)
		if readErr != nil {
			return readErr
		}
		if !found {
			return dal.ErrRecordNotFound
		}
		deleteErr := r.db.fileReader.deleteFile(ctx, recordPath, "ingitdb: delete "+colDef.ID+"/"+recordKey, sha)
		if deleteErr != nil {
			return deleteErr
		}
	}
	return nil
}

func (r readwriteTx) SetMulti(ctx context.Context, records []dal.Record) error {
	_, _ = ctx, records
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) DeleteMulti(ctx context.Context, keys []*dal.Key) error {
	_, _ = ctx, keys
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) Update(ctx context.Context, key *dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, key, updates, preconditions
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) UpdateRecord(ctx context.Context, record dal.Record, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, record, updates, preconditions
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) UpdateMulti(ctx context.Context, keys []*dal.Key, updates []update.Update, preconditions ...dal.Precondition) error {
	_, _, _, _ = ctx, keys, updates, preconditions
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) InsertMulti(ctx context.Context, records []dal.Record, opts ...dal.InsertOption) error {
	_, _, _ = ctx, records, opts
	return fmt.Errorf("not implemented by %s", DatabaseID)
}

func (r readwriteTx) ID() string {
	return ""
}

func encodeRecordContent(data map[string]any, format ingitdb.RecordFormat) ([]byte, error) {
	switch format {
	case ingitdb.RecordFormatYAML, ingitdb.RecordFormatYML:
		encoded, err := yaml.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode YAML record: %w", err)
		}
		return encoded, nil
	case ingitdb.RecordFormatJSON:
		encoded, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return nil, fmt.Errorf("failed to encode JSON record: %w", err)
		}
		return append(encoded, '\n'), nil
	case ingitdb.RecordFormatTOML:
		encoded, err := toml.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode TOML record: %w", err)
		}
		return encoded, nil
	case ingitdb.RecordFormatCSV:
		return nil, fmt.Errorf("encodeRecordContent does not support csv (single-record write path); csv requires record_type=[]map[string]any and the schema-aware EncodeRecordContentForCollection writer")
	default:
		return nil, fmt.Errorf("unsupported record format %q", format)
	}
}
