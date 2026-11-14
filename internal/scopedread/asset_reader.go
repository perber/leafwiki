package scopedread

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	coreassets "github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/tree"
)

// AssetFile is what a scoped caller gets of an asset: the name it asked for,
// the modification time for caching, and the content. The caller closes
// Content.
type AssetFile struct {
	Name    string
	ModTime time.Time
	Content io.ReadSeekCloser
}

// AssetOpener opens one asset of a page. *assets.AssetService implements it.
type AssetOpener interface {
	OpenAsset(pageID, filename string) (*os.File, error)
}

var (
	_ AssetOpener    = (*coreassets.AssetService)(nil)
	_ SubtreeChecker = (*tree.TreeService)(nil)
)

// AssetReader reads the assets of pages inside a ReadScope.
type AssetReader struct {
	pages SubtreeChecker
	files AssetOpener
}

// NewAssetReader returns an AssetReader that checks scopes against pages and
// opens files through files.
func NewAssetReader(pages SubtreeChecker, files AssetOpener) *AssetReader {
	return &AssetReader{pages: pages, files: files}
}

// Asset opens the asset filename of the page pageID if the page exists and is
// inside scope. The scope is checked before the file system is touched, so
// the assets of a page outside the scope stay unreachable even when a page
// inside it embeds them. Out-of-scope, unknown and deleted pages and missing
// or invalid file names are all ErrNotFound; operational errors are returned
// as they are and, like PageReader's, must not reach a scoped caller.
func (r *AssetReader) Asset(ctx context.Context, scope ReadScope, pageID, filename string) (*AssetFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := scope.validate(); err != nil {
		return nil, err
	}
	if err := scope.check(r.pages, pageID); err != nil {
		return nil, err
	}

	f, err := r.files.OpenAsset(pageID, filename)
	if errors.Is(err, coreassets.ErrAssetNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	// OpenAsset stats the file too, to refuse anything but a regular file;
	// keeping that inside core/assets is worth a second stat here.
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &AssetFile{Name: filename, ModTime: info.ModTime(), Content: f}, nil
}
