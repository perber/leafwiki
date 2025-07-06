package ssr

type Tabs struct {
	title string
	svg   string
}

type SidebarRenderer struct {
	TabsRenderer       *TabsRenderer
	NavigationRenderer *NavigationRenderer
}

func NewSidebarRenderer() *SidebarRenderer {
	return &SidebarRenderer{
		TabsRenderer:       NewTabsRenderer(),
		NavigationRenderer: NewNavigationRenderer(),
	}
}

func (r *SidebarRenderer) Render(tabs []Tabs, navigationItems []NavigationItem) string {
	if len(tabs) == 0 {
		return ""
	}

	result := "<div class=\"flex flex-col w-64 bg-white border-r border-gray-200\">"
	result += r.TabsRenderer.Render(tabs)
	result += "<div class=\"flex-1 overflow-y-auto\">"
	result += "<!-- Sidebar content goes here -->"
	result += r.NavigationRenderer.Render(navigationItems)
	result += "</div></div>"

	return result
}
