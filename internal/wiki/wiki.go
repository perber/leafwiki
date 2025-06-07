package wiki

import (
	"fmt"
	"log"
	"mime/multipart"
	"path"
	"regexp"

	"github.com/perber/wiki/internal/core/assets"
	"github.com/perber/wiki/internal/core/auth"
	"github.com/perber/wiki/internal/core/shared/errors"
	"github.com/perber/wiki/internal/core/tree"
	"github.com/perber/wiki/internal/search"
)

type Wiki struct {
	tree        *tree.TreeService
	slug        *tree.SlugService
	auth        *auth.AuthService
	user        *auth.UserService
	asset       *assets.AssetService
	searchIndex *search.SQLiteIndex
	status      *search.IndexingStatus
	storageDir  string
}

// Email-RegEx (Basic-Check, nicht RFC-konform, aber gut genug)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+$`)
var defaultAdminPassword = "admin"

func NewWiki(storageDir string, adminPassword string, jwtSecret string) (*Wiki, error) {
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

	sqliteIndex, err := search.NewSQLiteIndex(storageDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init search index: %w", err)
	}

	// status object for indexing
	status := search.NewIndexingStatus()

	// starts the indexing process in a separate goroutine
	go func() {
		err := search.BuildAndRunIndexer(treeService, sqliteIndex, path.Join(storageDir, "root"), 4, status)
		if err != nil {
			log.Printf("indexing failed: %v", err)
		}
	}()

	// Initialize the wiki service
	wiki := &Wiki{
		tree:        treeService,
		slug:        slugService,
		user:        userService,
		auth:        authService,
		asset:       assetService,
		storageDir:  storageDir,
		searchIndex: sqliteIndex,
		status:      status,
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

	return w.tree.GetPage(*id)
}

func (w *Wiki) UpdatePage(id, title, slug, content string) (*tree.Page, error) {
	err := w.tree.UpdatePage(id, title, slug, content)
	if err != nil {
		return nil, err
	}

	return w.tree.GetPage(id)
}

func (w *Wiki) DeletePage(id string, recursive bool) error {
	page, err := w.tree.GetPage(id)
	if err != nil {
		return err
	}

	if err := w.tree.DeletePage(id, recursive); err != nil {
		return err
	}

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

func (w *Wiki) SuggestSlug(parentID string, title string) (string, error) {
	// if no parentID is set or it's the root page
	// We don't need to look for a page id
	if parentID == "" || parentID == "root" {
		return w.slug.GenerateUniqueSlug(w.tree.GetTree(), title), nil
	}

	parent, err := w.tree.FindPageByID(w.tree.GetTree().Children, parentID)
	if err != nil {
		return "", fmt.Errorf("parent not found: %w", err)
	}

	return w.slug.GenerateUniqueSlug(parent, title), nil
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
	return w.searchIndex.Close()
}
