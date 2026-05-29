package dalgo2ingitdb

import (
	"os"

	"github.com/gofrs/flock"

	"github.com/ingitdb/ingitdb-cli/pkg/ingitdb/config"
)

// Test seams over os.*/config functions. These hold no state; tests swap them
// to inject failures that are otherwise unreachable (e.g. mkdir/write failures,
// or os.Remove returning ErrNotExist via a TOCTOU race), then restore them.
// A test that swaps a seam must NOT call t.Parallel(), since the swap mutates
// package-level state shared with other tests.
var (
	// osMkdirAll is used by CreateCollection.
	osMkdirAll = os.MkdirAll
	// osReadFile is used by rewriteRecordFiles.
	osReadFile = os.ReadFile
	// osWriteFile is used by writeCollectionDefYAML.
	osWriteFile = os.WriteFile
	// osRemove is used by deleteSingleRecordFile.
	osRemove = os.Remove
	// writeRootCollections is used by the registry helpers.
	writeRootCollections = config.WriteRootCollectionsToFile
	// readSingleRecord is used by readAllSingleRecords. The seam lets tests
	// reach the found==false branch, which in production only occurs when a
	// globbed file vanishes before the read (a TOCTOU race).
	readSingleRecord = readSingleRecordFile
	// newFileLocker is used by withSharedLock/withExclusiveLock. The seam lets
	// tests inject lock-acquisition failures. *flock.Flock satisfies fileLocker.
	newFileLocker = func(path string) fileLocker { return flock.New(path) }
)
