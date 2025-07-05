package ssr

type Tabs struct {
	title string
	svg   string
}

type TabsRenderer struct {
}

func NewTabsRenderer() *TabsRenderer {
	return &TabsRenderer{}
}

func (r *TabsRenderer) Render(tabs []Tabs) string {
	if len(tabs) == 0 {
		return ""
	}

	result := "<div class=\"pb-2 pt-2\">"
	result += "<div class=\"flex border-b text-sm\">"

	for _, tab := range tabs {
		result += "<button class=\"-mb-px flex items-center gap-1 border-b-2 px-3 py-1.5 "
		if tab.title == "Tree" {
			result += "border-green-600 font-semibold text-green-600"
		} else {
			result += "border-transparent text-gray-500 hover:text-black"
		}
		result += "\">"
		result += tab.svg
		result += tab.title
		result += "</button>"
	}

	result += "</div></div>"

	return result
}

type SidebarRenderer struct {
	TabsRenderer *TabsRenderer
}

func NewSidebarRenderer() *SidebarRenderer {
	return &SidebarRenderer{
		TabsRenderer: NewTabsRenderer(),
	}
}

func (r *SidebarRenderer) Render(tabs []Tabs) string {
	if len(tabs) == 0 {
		return ""
	}

	result := "<div class=\"flex flex-col w-64 bg-white border-r border-gray-200\">"
	result += r.TabsRenderer.Render(tabs)
	result += "<div class=\"flex-1 overflow-y-auto\">"
	result += "<!-- Sidebar content goes here -->"
	result += "</div></div>"

	return result
}
