package ingitdb

// MaterializeResult summarises the outcome of a materialisation run.
type MaterializeResult struct {
	FilesCreated   int
	FilesUpdated   int
	FilesUnchanged int
	FilesDeleted   int
	Errors         []error
}
