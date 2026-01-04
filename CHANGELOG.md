
# Changelog

This document records notable changes to LeafWiki, organized by release.

## ✅ v0.7.0 – Security, Authentication & UX Improvements

### Security & Authentication
- [x] Added session-based authentication backed by a local database
- [x] Secure, HttpOnly cookies enabled by default
- [x] CSRF protection for all state-changing requests
- [x] Rate limiting for authentication-related endpoints
- [x] Configurable access and refresh token timeouts
- [x] Added `--allow-insecure` flag to explicitly disable secure cookies (HTTP only)
- [x] Added `--disable-auth` flag to fully disable authentication (internal networks only)

### Access Control
- [x] Added **viewer role** for read-only access
- [x] Allow editing pages without login when authentication is disabled

### UI / UX Improvements
- [x] Sidebar is open by default
- [x] Improved image zoom behavior
- [x] Editor no longer loses content when switching modes
- [x] Hide login button when authentication is disabled

### Metadata & Content
- [x] Added metadata support for pages  (created at, updated at, author)

## ✅ v0.6.1

- [x] Changed default server host binding from `0.0.0.0` to `127.0.0.1` for safer local defaults (configure `--host` to expose externally)
- [x] Added `hide-link-metadata-section` flag to disable backlink section rendering
- [x] Allow some CSS attributes in markdown
- [x] Improved search ranking and use fuzzy search

## ✅ v0.6.0 – Backlink Support added

- [x] Added backlink support
- [x] Updated project dependencies
- [x] Fail on missing flag `--admin-password` to avoid accidental public exposure

## ✅ v0.5.2 – HTML Support in Markdown, Bugfixes and Dependency Updates

- [x] Add HTML support in Markdown pages - thanks @Hugo-Galley for the implementation!
- [x] Fixed an issue with links in the editor
- [x] Fixed print view for Dark mode
- [x] Updated project dependencies
- [x] Updated docker documentation in the readme - thanks @Hugo-Galley

## ✅ v0.5.0 – Dark mode, macOS Support and More

- [x] Dark mode support
- [x] Improve Docker labels and annotations - thanks @Hugo-Galley
- [x] macOS builds (x86_64 + arm64)
- [x] Anchor scrolling (jumping to headings in the page)
- [x] Various bug fixes and UX/UI improvements
- [x] Dependency updates across the project

## ✅ v0.4.10 – Clipboard Image/File Uploads, Resizable Sidebar and other UX Improvements

- [x] Docker images now have labels and annotations - thanks @Hugo-Galley
- [x] Installer now has a welcome message - thanks @Hugo-Galley
- [x] Allow to upload files by using **CTRL+V** in the codemirror editor
- [x] Improve position for tooltip in the treeview
- [x] Add toggle to **show & hide the preview**
- [x] Add **resizable sidebar** - thanks @magnus-madsen for the suggestion!
- [x] Various bug fixes and UX/UI improvements
- [x] Better e2e test coverage
- [x] Dependency updates across the project

## ✅ v0.4.9 – Mermaid Support, UX Improvements & Easier Installation

A special thanks for this release goes out to @Hugo-Galley.
He improved the documentation and the onboarding experience a lot! 

- [x] Mermaid.js diagram support
- [x] Copy page functionality added
- [x] Installation script added for binary - thanks to @Hugo-Galley
- [x] Improved docker builds with multi-arch support (amd64 + arm64)
- [x] Several UI/UX improvements and bugfixes
- [x] Stability improvements and dependency updates

## ✅ v0.4.8 – UX Improvements
- [x] Several dependencies updates
- [x] Not Found page now suggests creating a new page - thanks @magnus-madsen for the suggestion!
- [x] links to non-existing pages now show a create page dialog - thanks @magnus-madsen for the suggestion!
- [x] smaller UI improvements and bugfixes (e.g. green save button, ...)

## ✅ v0.4.7 – Stabilize
- [x] Several dependencies updates
- [x] Allow to configure `--host` to bind to specific IP (e.g. `--host 127.0.0.1`) - thanks @magnus-madsen for the suggestion!

## ✅ v0.4.6 – Ready for Dogfooding
- [x] Add Search functionality for page titles and content
- [x] Add Mobile optimizations for better usability
- [x] Allow Public Pages (viewable pages without login)
- [x] Add shortcuts in the editor (e.g. Ctrl+S to save, Ctrl+B for bold, Ctrl+Z for undo, ...)
- [x] Smaller improvements and bugfixes in the UI
- [x] Added "Create & Edit" option to dialog to allow creating structure before editing
- [x] Warn user about unsaved changes when navigating away (via `beforeunload` and `react-router`)
- [x] Updated the tree view design – it now has a more documentation-style look
- [x] Print view support for pages (print-friendly layout)

## ✅ v0.3.4 – Improved Asset Handling
- [x] Allow uploading multiple files at once
- [x] Allow renaming of uploaded files
- [x] Fix caching issues with uploaded assets
- [x] Fix syntax highlighting in preview
- [x] Fix favicon not displayed
- [x] ARM64 support for Raspberry Pi and other ARM devices (thanks @nahaktarun)

## ✅ v0.2.0 – Improved Editor Experience
- [x] Use CodeMirror for Markdown editing
- [x] Add Toolbar with common actions like bold, italic, links, etc.
- [x] Allow Undo/Redo actions

## ✅ v0.1.0 – MVP
- [x] Tree-based page structure
- [x] Markdown file creation
- [x] Slug + file path mapping
- [x] Move / rename / delete logic
- [x] Markdown editor with preview
- [x] File/image uploads per page
- [x] Simple page title search
- [x] Asset management (images, files)
- [x] Basic JWT auth (session-based)

