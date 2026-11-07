package mcp

import (
	"context"
	"log/slog"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	coreauth "github.com/perber/wiki/internal/core/auth"
)

// newPatchPageTool registers the patch_page tool: it replaces line-numbered
// ranges of a page's body with replacement text (an empty replacement
// deletes the lines) and returns the patched page. The format is documented
// in the tool description so an agent can build a patch from the line numbers
// get_page renders without another round trip. The role gate mirrors the REST
// update route (a patch is a content update), which requires editor.
func newPatchPageTool(log *slog.Logger, ops PageOps) mcpserver.ServerTool {
	tool := mcpgo.NewTool("patch_page",
		mcpgo.WithDescription(
			"Replace ranges of a page's markdown body by line number and return the patched page. "+
				"The line numbers are the 1-based, inclusive numbers get_page renders. "+
				"The patch argument is a string containing a JSON array of range objects: "+
				`[{"start": 2, "end": 3, "replace": "new line one\nnew line two"}, {"start": 7, "end": 7, "replace": ""}]. `+
				"Each range covers lines start through end; replace is the text that takes the range's place "+
				"(it may span multiple lines; an empty string deletes the lines). "+
				"Ranges must not overlap and must not extend beyond the end of the page — read the page with get_page first. "+
				"Line numbers follow get_page's rendering, so content ending in a newline has an empty final line, "+
				"and a replacement ending in \n adds one. "+
				"Requires editor or admin role.",
		),
		mcpgo.WithString("id",
			mcpgo.Required(),
			mcpgo.Description("The stable page ID"),
		),
		mcpgo.WithString("patch",
			mcpgo.Required(),
			mcpgo.Description(`The patch as a string containing a JSON array of {"start","end","replace"} range objects (see the tool description)`),
		),
	)
	return mcpserver.ServerTool{
		Tool: tool,
		Handler: func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			if toolErr := requireRole(ctx, coreauth.RoleEditor); toolErr != nil {
				return toolResult(log, tool.Name, toolErr), nil
			}
			id, message := pageIDArg(req)
			patch, patchMessage := patchArg(req)
			if message == "" {
				message = patchMessage
			}
			if message != "" {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: message}), nil
			}
			ranges, err := parsePatch(patch)
			if err != nil {
				return toolResult(log, tool.Name, &ToolError{Code: ErrCodeMCPInvalidInput, Message: err.Error()}), nil
			}
			page, err := ops.PatchPage(ctx, PatchPageInput{Actor: actorFromContext(ctx), ID: id, Ranges: ranges})
			if err != nil {
				return toolResult(log, tool.Name, toolErrorFrom(err)), nil
			}
			return mcpgo.NewToolResultText(renderPage(page)), nil
		},
	}
}

// patchArg extracts the patch argument, which must be a string containing a
// JSON array of range objects. An agent that sends a real JSON array (the
// tempting shape) would read back as an empty string from GetString and get a
// misleading "patch is required", so a present-but-non-string value gets the
// precise answer instead.
func patchArg(req mcpgo.CallToolRequest) (string, string) {
	arg, present := req.GetArguments()["patch"]
	if !present {
		return "", "patch is required"
	}
	patch, ok := arg.(string)
	if !ok {
		return "", "patch must be a string containing a JSON array of range objects"
	}
	if patch == "" {
		return "", "patch is required"
	}
	return patch, ""
}
