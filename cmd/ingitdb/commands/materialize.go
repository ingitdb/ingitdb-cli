package commands

// specscore: feature/cli/materialize

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/ingitdb/ingitdb-go"
	"github.com/ingitdb/ingitdb-go/docsbuilder"
	"github.com/ingitdb/ingitdb-go/gitrepo"
	"github.com/ingitdb/ingitdb-go/materializer"
)

func materializeCommandRunE(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, _ []string) error {
		if viewBuilder == nil {
			return fmt.Errorf("not yet implemented")
		}

		dirPath, err := resolveMaterializePath(cmd, homeDir, getWd)
		if err != nil {
			return err
		}
		logf("inGitDB db path: ", dirPath)

		ctx := cmd.Context()
		repoRoot, err := gitrepo.FindRepoRoot(dirPath)
		if err != nil {
			logf(fmt.Sprintf("Could not find git repository root for default view export: %v", err))
			repoRoot = ""
		}

		def, err := readDefinition(dirPath)
		if err != nil {
			return fmt.Errorf("failed to read database definition: %w", err)
		}

		colsRaw, _ := cmd.Flags().GetString("collections")
		viewsRaw, _ := cmd.Flags().GetString("views")
		colsSel := flagSelection(cmd.Flags().Changed("collections"), colsRaw)
		viewsSel := flagSelection(cmd.Flags().Changed("views"), viewsRaw)

		// Bare `materialize` (neither flag) regenerates everything.
		if colsSel.kind == selectionNone && viewsSel.kind == selectionNone {
			colsSel = selection{kind: selectionAll}
			viewsSel = selection{kind: selectionAll}
		}

		var totalResult ingitdb.MaterializeResult

		if colsSel.kind != selectionNone {
			colResult, colErr := materializeCollections(ctx, def, dirPath, colsSel)
			if colErr != nil {
				return colErr
			}
			mergeMaterializeResult(&totalResult, colResult)
		}

		if viewsSel.kind != selectionNone {
			var recordsDelimiter *int
			if cmd.Flags().Changed("records-delimiter") {
				v, _ := cmd.Flags().GetInt("records-delimiter")
				recordsDelimiter = &v
			}
			def.RuntimeOverrides.RecordsDelimiter = recordsDelimiter

			viewResult, viewErr := materializeViews(ctx, viewBuilder, def, dirPath, repoRoot, viewsSel)
			if viewErr != nil {
				return viewErr
			}
			mergeMaterializeResult(&totalResult, viewResult)
		}

		logf(materializeSummary(&totalResult))
		return nil
	}
}

// materializeCollections regenerates collection READMEs through docsbuilder.
// For selectionAll it uses the "**" glob; for selectionList it runs once per
// pattern and merges the results.
func materializeCollections(
	ctx context.Context,
	def *ingitdb.Definition,
	dirPath string,
	sel selection,
) (*ingitdb.MaterializeResult, error) {
	reader := materializer.NewFileRecordsReader()
	var globs []string
	if sel.kind == selectionAll {
		globs = []string{"**"}
	} else {
		globs = sel.globs
	}
	total := &ingitdb.MaterializeResult{}
	for _, glob := range globs {
		result, err := docsbuilder.UpdateDocs(ctx, def, glob, dirPath, reader)
		if err != nil {
			return nil, fmt.Errorf("failed to regenerate collection READMEs for %q: %w", glob, err)
		}
		mergeMaterializeResult(total, result)
	}
	return total, nil
}

// materializeViews regenerates materialized views through the view builder.
// For selectionAll it rebuilds every view of every collection (BuildViews). For
// selectionList it rebuilds only the views whose names match the glob list,
// leaving non-matching view output files untouched.
func materializeViews(
	ctx context.Context,
	viewBuilder materializer.ViewBuilder,
	def *ingitdb.Definition,
	dirPath string,
	repoRoot string,
	sel selection,
) (*ingitdb.MaterializeResult, error) {
	total := &ingitdb.MaterializeResult{}

	if sel.kind == selectionAll {
		for _, col := range eachCollection(def.Collections) {
			result, err := viewBuilder.BuildViews(ctx, dirPath, repoRoot, col, def)
			if err != nil {
				return nil, fmt.Errorf("failed to materialize views for collection %s: %w", col.ID, err)
			}
			mergeMaterializeResult(total, result)
		}
		return total, nil
	}

	// selectionList: build only the views whose names match the glob list. We
	// rely on the per-view BuildView entry point so non-matching views are never
	// re-rendered, satisfying the views-subset contract.
	builder, ok := viewBuilder.(materializer.SimpleViewBuilder)
	if !ok {
		return nil, fmt.Errorf("view-name filtering requires the standard view builder")
	}
	for _, col := range eachCollection(def.Collections) {
		names := sortedViewNames(col.Views)
		matched := matchViewNames(names, sel.globs)
		for _, name := range matched {
			view := col.Views[name]
			result, err := builder.BuildView(ctx, dirPath, repoRoot, col, def, view)
			if err != nil {
				return nil, fmt.Errorf("failed to materialize view %s/%s: %w", col.ID, name, err)
			}
			mergeMaterializeResult(total, result)
		}
	}
	return total, nil
}

// eachCollection flattens the collection tree (top-level plus all nested
// subcollections) into a single slice for iteration.
func eachCollection(collections map[string]*ingitdb.CollectionDef) []*ingitdb.CollectionDef {
	var out []*ingitdb.CollectionDef
	for _, col := range collections {
		out = append(out, col)
		out = append(out, eachCollection(col.SubCollections)...)
	}
	return out
}

// sortedViewNames returns the view IDs of a collection in deterministic order.
func sortedViewNames(views map[string]*ingitdb.ViewDef) []string {
	names := make([]string, 0, len(views))
	for name := range views {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// resolveMaterializePath resolves the --path flag (or working directory) into an
// absolute, home-expanded database directory path.
func resolveMaterializePath(
	cmd *cobra.Command,
	homeDir func() (string, error),
	getWd func() (string, error),
) (string, error) {
	dirPath, _ := cmd.Flags().GetString("path")
	if dirPath == "" {
		wd, err := getWd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
		dirPath = wd
	}
	expanded, err := expandHome(dirPath, homeDir)
	if err != nil {
		return "", err
	}
	abs, _ := filepath.Abs(expanded)
	return abs, nil
}

// mergeMaterializeResult accumulates src into dst.
func mergeMaterializeResult(dst *ingitdb.MaterializeResult, src *ingitdb.MaterializeResult) {
	if src == nil {
		return
	}
	dst.FilesCreated += src.FilesCreated
	dst.FilesUpdated += src.FilesUpdated
	dst.FilesUnchanged += src.FilesUnchanged
	dst.FilesDeleted += src.FilesDeleted
	dst.Errors = append(dst.Errors, src.Errors...)
}

// materializeSummary renders the created/updated/deleted/unchanged tally line.
func materializeSummary(r *ingitdb.MaterializeResult) string {
	return fmt.Sprintf("materialized: %d created, %d updated, %d deleted, %d unchanged",
		r.FilesCreated, r.FilesUpdated, r.FilesDeleted, r.FilesUnchanged)
}

// Materialize returns the materialize command.
func Materialize(
	homeDir func() (string, error),
	getWd func() (string, error),
	readDefinition func(string, ...ingitdb.ReadOption) (*ingitdb.Definition, error),
	viewBuilder materializer.ViewBuilder,
	logf func(...any),
) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "materialize",
		Short: "Regenerate derived artifacts: collection READMEs and materialized views",
		RunE:  materializeCommandRunE(homeDir, getWd, readDefinition, viewBuilder, logf),
	}
	addMaterializeCommandFlags(cmd)
	return cmd
}
