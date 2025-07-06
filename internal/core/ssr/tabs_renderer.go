package ssr

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
