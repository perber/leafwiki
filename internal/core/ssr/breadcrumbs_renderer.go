package ssr

type Breadcrumb struct {
	Title string
	URL   string
}

type BreadcrumbsRenderer struct {
}

func NewBreadcrumbsRenderer() *BreadcrumbsRenderer {
	return &BreadcrumbsRenderer{}
}

func (r *BreadcrumbsRenderer) Render(breadcrumbs []Breadcrumb) string {
	if len(breadcrumbs) == 0 {
		return ""
	}

	result := "<nav aria-label=\"breadcrumb\" class=\"flex w-full flex-1 flex-grow text-sm text-gray-500\">"
	result += "<ol class=\"flex flex-wrap items-center gap-1\">"
	for index, crumb := range breadcrumbs {
		result += "<li class=\"flex items-center gap-1\">"
		if (len(breadcrumbs) - 1) == index {
			result += "<span class=\"font-semibold text-gray-700\">" + crumb.Title + "</span>"
		} else {
			result += "<span>/</span>"
			result += "<a class=\"text-gray-700 hover:underline\" href=\"/" + crumb.URL + "\" data-discover=\"true\">" + crumb.Title + "</a>"
		}
		result += "</li>"
	}
	result += "</ol></nav>"

	return result
}
