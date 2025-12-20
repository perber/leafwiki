package wiki

import (
	"fmt"
	"log"
	"mime/multipart"
	"path"
	"regexp"
	"strings"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/links"
	"github.com/perber/wiki/internal/search"
)

type Wiki struct {
	tree          *tree.TreeService
	slug          *tree.SlugService
	auth          *auth.AuthService
	user          *auth.UserService
	asset         *assets.AssetService
	searchIndex   *search.SQLiteIndex
	status        *search.IndexingStatus
	storageDir    string
	searchWatcher *search.Watcher
	links         *links.LinkService
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+$`)
var defaultAdminPassword = "admin"

func NewWiki(storageDir string, adminPassword string, jwtSecret string, enableSearchIndexing bool) (*Wiki, error) {
	// Initialize the user store
	store, err := auth.NewUserStore(storageDir)
	if err != nil {
		return nil, err
	}

	if adminPassword == "" {
		adminPassword = defaultAdminPassword
	}

	// Initialize the user service
	userService := auth.NewUserService(store)
	if err := userService.InitDefaultAdmin(adminPassword); err != nil {
		return nil, err
	}

	// Initialize the auth service
	authService := auth.NewAuthService(userService, jwtSecret)

	// Initialize the tree service
	treeService := tree.NewTreeService(storageDir)
	if err := treeService.LoadTree(); err != nil {
		return nil, err
	}

	slugService := tree.NewSlugService()

	assetService := assets.NewAssetService(storageDir, slugService)

	// Backlink Service
	linksStore, err := links.NewLinksStore(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init links store: %w", err)
	}
	linkService := links.NewLinkService(storageDir, treeService, linksStore)
	if err := linkService.IndexAllPages(); err != nil {
		log.Printf("failed to index links of pages: %v", err)
	}

	sqliteIndex, err := search.NewSQLiteIndex(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init search index: %w", err)
	}

	// status object for indexing
	status := search.NewIndexingStatus()

	var searchWatcher *search.Watcher
	if enableSearchIndexing {
		// starts the indexing process in a separate goroutine
		go func() {
			err := search.BuildAndRunIndexer(treeService, sqliteIndex, path.Join(storageDir, "root"), 4, status)
			if err != nil {
				log.Printf("indexing failed: %v", err)
			}
		}()

		// Start the file watcher for indexing
		var err error
		searchWatcher, err = search.NewWatcher(path.Join(storageDir, "root"), treeService, sqliteIndex, status)
		if err != nil {
			log.Printf("failed to create file watcher: %v", err)
		} else {
			go func() {
				if err := searchWatcher.Start(); err != nil {
					log.Printf("failed to start file watcher: %v", err)
				}
			}()
		}
	}

	// Initialize the wiki service
	wiki := &Wiki{
		tree:          treeService,
		slug:          slugService,
		user:          userService,
		auth:          authService,
		asset:         assetService,
		storageDir:    storageDir,
		searchIndex:   sqliteIndex,
		status:        status,
		searchWatcher: searchWatcher,
		links:         linkService,
	}

	// Ensure the welcome page exists
	if err := wiki.EnsureWelcomePage(); err != nil {
		return nil, err
	}

	return wiki, nil
}

func (w *Wiki) EnsureWelcomePage() error {
	_, err := w.tree.GetPage("root")
	if err == nil {
		return nil
	}

	if len(w.tree.GetTree().Children) > 0 {
		return nil
	}

	p, err := w.CreatePage(nil, "Welcome to Leaf Wiki", "welcome-to-leaf-wiki")
	if err != nil {
		return err
	}

	// Set the content of the welcome page
	content := `# Welcome to LeafWiki!

This is your personal, lightweight Markdown wiki.  
You can write, edit, and structure pages – all in a simple tree layout.

---

## Features

- **Live Markdown editor** with preview
- **Tree-based navigation**
- **Per-page assets** like images and files
- **No database** – just clean files
- **Single Go binary** – easy to run

---

## Tips

- Click the + button to create new pages or folders
- Double-click an asset to insert it into the editor
- Use Markdown for formatting, like:

` + "```" + `md
- Lists
- **Bold**
` + "- `Inline code` \n```\n\n" + "Enjoy writing!"

	if _, err := w.UpdatePage(p.ID, p.Title, p.Slug, content); err != nil {
		return err
	}

	return nil
}

func (w *Wiki) GetTree() *tree.PageNode {
	return w.tree.GetTree()
}

func (w *Wiki) CreatePage(parentID *string, title string, slug string) (*tree.Page, error) {
	ve := errors.NewValidationErrors()

	if title == "" {
		ve.Add("title", "Title must not be empty")
	}

	if err := w.slug.IsValidSlug(slug); err != nil {
		ve.Add("slug", err.Error())
	}

	if ve.HasErrors() {
		return nil, ve
	}

	// Check if the parentID exists
	if parentID != nil && *parentID != "" {
		var err error
		_, err = w.tree.FindPageByID(w.tree.GetTree().Children, *parentID)
		if err != nil {
			return nil, err
		}
	}

	id, err := w.tree.CreatePage(parentID, title, slug)
	if err != nil {
		return nil, err
	}

	page, err := w.tree.GetPage(*id)
	if err != nil {
		return nil, err
	}

	if w.links != nil {
		if err := w.links.HealOnPageCreate(page); err != nil {
			log.Printf("warning: failed to heal links for page %s: %v", page.ID, err)
		}
	}

	return page, nil
}

func (w *Wiki) EnsurePath(targetPath string, targetTitle string) (*tree.Page, error) {
	ve := errors.NewValidationErrors()

	cleanTargetPath := strings.Trim(strings.TrimSpace(targetPath), "/")
	if cleanTargetPath == "" {
		ve.Add("path", "Path must not be empty")
	}

	cleanTargetTitle := strings.TrimSpace(targetTitle)
	if cleanTargetTitle == "" {
		ve.Add("title", "Title must not be empty")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	lookup, err := w.tree.LookupPagePath(w.tree.GetTree().Children, cleanTargetPath)
	if err != nil {
		return nil, err
	}

	// If the path exists, return the last page
	if lookup.Exists {
		return w.GetPage(*lookup.Segments[len(lookup.Segments)-1].ID)
	}

	// Check if not existing segments have a valid slug
	for _, segment := range lookup.Segments {
		if !segment.Exists {
			if err := w.slug.IsValidSlug(segment.Slug); err != nil {
				ve.Add("path", fmt.Sprintf("Invalid slug '%s': %s", segment.Slug, err.Error()))
			}
		}
	}

	if ve.HasErrors() {
		return nil, ve
	}

	// Now we create the missing segments
	result, err := w.tree.EnsurePagePath(cleanTargetPath, cleanTargetTitle)
	if err != nil {
		return nil, err
	}

	page, err := w.tree.GetPage(result.Page.ID)
	if err != nil {
		return nil, err
	}

	if w.links != nil {
		for _, n := range result.Created {
			p, err := w.tree.GetPage(n.ID)
			if err != nil {
				log.Printf("warning: failed to get page %s for healing links: %v", n.ID, err)
				continue
			}
			if err := w.links.HealOnPageCreate(p); err != nil {
				log.Printf("warning: failed to heal links for page %s: %v", n.ID, err)
			}
		}
	}

	return page, nil
}

func (w *Wiki) UpdatePage(id, title, slug, content string) (*tree.Page, error) {

	// Validate the request
	ve := errors.NewValidationErrors()
	if title == "" {
		ve.Add("title", "Title must not be empty")
	}
	if err := w.slug.IsValidSlug(slug); err != nil {
		ve.Add("slug", err.Error())
	}
	if ve.HasErrors() {
		return nil, ve
	}

	err := w.tree.UpdatePage(id, title, slug, content)
	if err != nil {
		return nil, err
	}

	page, err := w.tree.GetPage(id)
	if err != nil {
		return page, err
	}

	if w.links != nil {
		if err := w.links.UpdateLinksForPage(page, content); err != nil {
			log.Printf("warning: failed to update backlinks for page %s: %v", id, err)
		}
	}

	return page, nil
}

func (w *Wiki) CopyPage(currentPageID string, targetParentID *string, title string, slug string) (*tree.Page, error) {
	// Validate the request
	ve := errors.NewValidationErrors()
	if title == "" {
		ve.Add("title", "Title must not be empty")
	}
	if err := w.slug.IsValidSlug(slug); err != nil {
		ve.Add("slug", err.Error())
	}
	if ve.HasErrors() {
		return nil, ve
	}

	// Find the current page
	page, err := w.tree.GetPage(currentPageID)
	if err != nil {
		return nil, err
	}

	// Create a copy of the page
	copyID, err := w.tree.CreatePage(targetParentID, title, slug)
	if err != nil {
		return nil, err
	}
	cleanup := func() { _ = w.tree.DeletePage(*copyID, false) }

	// Get the copied page
	copy, err := w.tree.GetPage(*copyID)
	if err != nil {
		cleanup()
		return nil, err
	}

	// Copy assets!
	if err := w.asset.CopyAllAssets(page.PageNode, copy.PageNode); err != nil {
		cleanup()
		return nil, err
	}

	// Update the content to point to the new asset paths
	updatedContent := strings.ReplaceAll(page.Content, "/assets/"+page.ID+"/", "/assets/"+copy.ID+"/")

	// Write the content to the copied page
	if err := w.tree.UpdatePage(copy.ID, copy.Title, copy.Slug, updatedContent); err != nil {
		cleanup()
		_ = w.asset.DeleteAllAssetsForPage(copy.PageNode)
		return nil, err
	}

	if w.links != nil {
		if err := w.links.HealOnPageCreate(copy); err != nil {
			log.Printf("warning: failed to heal links for page %s: %v", copy.ID, err)
		}
	}

	return copy, nil
}

func (w *Wiki) DeletePage(id string, recursive bool) error {
	if id == "root" || id == "" {
		return fmt.Errorf("cannot delete root page")
	}

	collectSubtreeIDs := func(node *tree.PageNode) []string {
		var ids []string
		var walk func(n *tree.PageNode)
		walk = func(n *tree.PageNode) {
			if n == nil {
				return
			}
			if n.ID != "root" {
				ids = append(ids, n.ID)
			}
			for _, c := range n.Children {
				walk(c)
			}
		}
		walk(node)
		return ids
	}

	page, err := w.tree.GetPage(id)
	if err != nil {
		return err
	}

	var subtreeIDs []string
	var oldPrefix string

	if recursive {
		root := w.tree.GetTree()
		if root != nil {
			node, err := w.tree.FindPageByID(root.Children, id)
			if err == nil && node != nil {
				subtreeIDs = collectSubtreeIDs(node)
				oldPrefix = node.CalculatePath() // IMPORTANT: before delete
			}
		}
	}

	// If recursive, also handle subtree
	// we need to mark links broken for all pages in the subtree
	// Delete assets for subtree pages as well
	if recursive {

		// No pages in subtree or empty oldPrefix
		// add current page id to subtreeIDs to delete its links and assets as well
		if len(subtreeIDs) == 0 || oldPrefix == "" {
			subtreeIDs = []string{id}
			oldPrefix = page.CalculatePath()
		}

		if err := w.tree.DeletePage(id, recursive); err != nil {
			return err
		}

		if w.links != nil {
			for _, pid := range subtreeIDs {
				if err := w.links.DeleteOutgoingLinksForPage(pid); err != nil {
					log.Printf("warning: could not delete outgoing links for page %s: %v", pid, err)
				}
			}
			if oldPrefix != "" {
				if err := w.links.MarkLinksBrokenForPrefix(oldPrefix); err != nil {
					log.Printf("warning: could not mark links broken for prefix %s: %v", oldPrefix, err)
				}
			}
		}

		// Delete assets for all pages in the subtree
		for _, pid := range subtreeIDs {
			if err := w.asset.DeleteAllAssetsForPage(&tree.PageNode{ID: pid}); err != nil {
				log.Printf("warning: could not delete assets for page %s: %v", pid, err)
			}
		}

		return nil
	}

	if err := w.tree.DeletePage(id, recursive); err != nil {
		return err
	}

	// non-recursive case
	if w.links != nil {
		// Finally, mark incoming links as broken
		if err := w.links.DeleteOutgoingLinksForPage(id); err != nil {
			log.Printf("warning: could not delete outgoing links for page %s: %v", id, err)
		}
		if err := w.links.MarkIncomingLinksBrokenForPage(id); err != nil {
			log.Printf("warning: could not mark incoming links broken for page %s: %v", id, err)
		}
		if err := w.links.MarkLinksBrokenForPath(page.CalculatePath()); err != nil {
			log.Printf("warning: could not mark links broken for path %s: %v", page.CalculatePath(), err)
		}
	}

	// Also delete all assets for the page
	if err := w.asset.DeleteAllAssetsForPage(page.PageNode); err != nil {
		log.Printf("warning: could not delete assets for page %s: %v", page.ID, err)
	}

	return nil
}

func (w *Wiki) MovePage(id, parentID string) error {
	return w.tree.MovePage(id, parentID)
}

func (w *Wiki) SortPages(parentID string, orderedIDs []string) error {
	return w.tree.SortPages(parentID, orderedIDs)
}

func (w *Wiki) GetPage(id string) (*tree.Page, error) {
	return w.tree.GetPage(id)
}

func (w *Wiki) FindByPath(route string) (*tree.Page, error) {
	return w.tree.FindPageByRoutePath(w.tree.GetTree().Children, route)
}

func (w *Wiki) LookupPagePath(path string) (*tree.PathLookup, error) {
	return w.tree.LookupPagePath(w.tree.GetTree().Children, path)
}

func (w *Wiki) SuggestSlug(parentID string, currentID string, title string) (string, error) {
	// if no parentID is set or it's the root page
	// We don't need to look for a page id
	if parentID == "" || parentID == "root" {
		return w.slug.GenerateUniqueSlug(w.tree.GetTree(), currentID, title), nil
	}

	parent, err := w.tree.FindPageByID(w.tree.GetTree().Children, parentID)
	if err != nil {
		return "", fmt.Errorf("parent not found: %w", err)
	}

	return w.slug.GenerateUniqueSlug(parent, currentID, title), nil
}

func (w *Wiki) ReindexBacklinks() error {
	if w.links == nil {
		return nil
	}
	return w.links.IndexAllPages()
}

func (w *Wiki) GetBacklinks(pageID string) (*links.BacklinkResult, error) {
	if w.links == nil {
		return nil, fmt.Errorf("links not available")
	}
	return w.links.GetBacklinksForPage(pageID)
}

func (w *Wiki) GetOutgoingLinks(pageID string) (*links.OutgoingResult, error) {
	if w.links == nil {
		return nil, fmt.Errorf("outgoing links not available")
	}
	return w.links.GetOutgoingLinksForPage(pageID)
}

func (w *Wiki) Login(identifier, password string) (*auth.AuthToken, error) {
	return w.auth.Login(identifier, password)
}

func (w *Wiki) RefreshToken(token string) (*auth.AuthToken, error) {
	return w.auth.RefreshToken(token)
}

func (w *Wiki) CreateUser(username, email, password, role string) (*auth.PublicUser, error) {
	ve := errors.NewValidationErrors()
	if username == "" {
		ve.Add("username", "Username must not be empty")
	}
	if email == "" {
		ve.Add("email", "Email must not be empty")
	} else if !emailRegex.MatchString(email) {
		ve.Add("email", "Email is not valid")
	}
	if password == "" {
		ve.Add("password", "Password must not be empty")
	} else if len(password) < 8 {
		ve.Add("password", "Password must be at least 8 characters long")
	}
	if !auth.IsValidRole(role) {
		ve.Add("role", "Invalid role")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	user, err := w.user.CreateUser(username, email, password, role)
	if err != nil {
		return nil, err
	}

	return user.ToPublicUser(), nil
}

func (w *Wiki) UpdateUser(id, username, email, password, role string) (*auth.PublicUser, error) {

	ve := errors.NewValidationErrors()
	if username == "" {
		ve.Add("username", "Username must not be empty")
	}
	if email == "" {
		ve.Add("email", "Email must not be empty")
	} else if !emailRegex.MatchString(email) {
		ve.Add("email", "Email is not valid")
	}
	if !auth.IsValidRole(role) {
		ve.Add("role", "Invalid role")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	user, err := w.user.UpdateUser(id, username, email, password, role)
	if err != nil {
		return nil, err
	}

	return user.ToPublicUser(), nil
}

func (w *Wiki) ChangeOwnPassword(id, oldPassword, newPassword string) error {
	ve := errors.NewValidationErrors()
	if newPassword == "" {
		ve.Add("newPassword", "New password must not be empty")
	} else if len(newPassword) < 8 {
		ve.Add("newPassword", "New password must be at least 8 characters long")
	}

	_, err := w.GetUserService().DoesIDAndPasswordMatch(id, oldPassword)
	if err != nil {
		ve.Add("oldPassword", "Old password is incorrect")
	}

	if ve.HasErrors() {
		return ve
	}

	return w.user.ChangeOwnPassword(id, oldPassword, newPassword)
}

func (w *Wiki) DeleteUser(id string) error {
	return w.user.DeleteUser(id)
}

func (w *Wiki) UpdatePassword(id, password string) error {
	ve := errors.NewValidationErrors()
	if password == "" {
		ve.Add("password", "Password must not be empty")
	} else if len(password) < 8 {
		ve.Add("password", "Password must be at least 8 characters long")
	}

	if ve.HasErrors() {
		return ve
	}

	return w.user.UpdatePassword(id, password)
}

func (w *Wiki) GetUsers() ([]*auth.PublicUser, error) {
	users, err := w.user.GetUsers()
	if err != nil {
		return nil, err
	}

	publicUsers := make([]*auth.PublicUser, len(users))
	for i, user := range users {
		publicUsers[i] = user.ToPublicUser()
	}

	return publicUsers, nil
}

func (w *Wiki) GetUserByID(id string) (*auth.PublicUser, error) {
	user, err := w.user.GetUserByID(id)
	if err != nil {
		return nil, err
	}

	return user.ToPublicUser(), nil
}

func (w *Wiki) ResetAdminUserPassword() (*auth.User, error) {
	adminUser, err := w.user.ResetAdminUserPassword()
	if err != nil {
		return nil, err
	}

	return adminUser, nil
}

func (w *Wiki) UploadAsset(pageID string, file multipart.File, filename string) (string, error) {
	page, err := w.tree.FindPageByID(w.tree.GetTree().Children, pageID)
	if err != nil {
		return "", err
	}
	return w.asset.SaveAssetForPage(page, file, filename)
}

func (w *Wiki) ListAssets(pageID string) ([]string, error) {
	page, err := w.tree.FindPageByID(w.tree.GetTree().Children, pageID)
	if err != nil {
		return nil, err
	}
	return w.asset.ListAssetsForPage(page)
}

func (w *Wiki) RenameAsset(pageID string, oldFilename, newFilename string) (string, error) {
	page, err := w.tree.FindPageByID(w.tree.GetTree().Children, pageID)
	if err != nil {
		return "", err
	}
	return w.asset.RenameAsset(page, oldFilename, newFilename)
}

func (w *Wiki) DeleteAsset(pageID string, filename string) error {
	page, err := w.tree.FindPageByID(w.tree.GetTree().Children, pageID)
	if err != nil {
		return err
	}
	return w.asset.DeleteAsset(page, filename)
}

func (w *Wiki) GetIndexingStatus() *search.IndexingStatus {
	return w.status.Snapshot()
}

func (w *Wiki) IsIndexingActive() bool {
	return w.status != nil && w.status.IsActive()
}

func (w *Wiki) Search(query string, offset, limit int) (*search.SearchResult, error) {
	if w.searchIndex == nil {
		return nil, fmt.Errorf("search index not available")
	}
	return w.searchIndex.Search(query, offset, limit)
}

func (w *Wiki) GetUserService() *auth.UserService {
	return w.user
}

func (w *Wiki) GetAuthService() *auth.AuthService {
	return w.auth
}

func (w *Wiki) GetAssetService() *assets.AssetService {
	return w.asset
}

func (w *Wiki) GetStorageDir() string {
	return w.storageDir
}

func (w *Wiki) Close() error {
	w.status.Finish()
	if err := w.user.Close(); err != nil {
		return err
	}

	if w.links != nil {
		if err := w.links.Close(); err != nil {
			log.Printf("error closing links: %v", err)
		}
	}

	if w.searchWatcher != nil {
		if err := w.searchWatcher.Stop(); err != nil {
			log.Printf("error stopping search watcher: %v", err)
		}
	}

	return w.searchIndex.Close()
}
