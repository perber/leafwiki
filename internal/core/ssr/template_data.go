package ssr

import "html/template"

type TemplateData struct {
	Title       string
	Description string
	Content     template.HTML
	Breadcrumbs template.HTML
	Sidebar     template.HTML
}
