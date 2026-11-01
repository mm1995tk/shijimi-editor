package core

import (
	"path/filepath"
	"strings"
)

// fileIcon is a Nerd Font glyph and its color. kind names the icon so a theme
// can override it (see Theme.Icons).
type fileIcon struct {
	kind  string
	Glyph string
	Color string
}

var (
	iconFolder     = fileIcon{"folder", "", "#7aa2c8"}
	iconFolderOpen = fileIcon{"folder-open", "", "#7aa2c8"}
	iconDefault    = fileIcon{"default", "", "#a0a0a0"}
)

var (
	IcoGo       = fileIcon{"go", "", "#00add8"}
	icoNix      = fileIcon{"nix", "", "#7ebae4"}
	icoMarkdown = fileIcon{"markdown", "", "#519aba"}
	icoImage    = fileIcon{"image", "", "#a074c4"}
	icoJSON     = fileIcon{"json", "", "#cbcb41"}
	icoConfig   = fileIcon{"config", "", "#9aa5b1"}
	icoShell    = fileIcon{"shell", "", "#89e051"}
	icoLock     = fileIcon{"lock", "", "#9aa5b1"}
	icoGit      = fileIcon{"git", "", "#f1502f"}
	icoJS       = fileIcon{"js", "", "#cbcb41"}
	icoTS       = fileIcon{"ts", "", "#519aba"}
	icoPython   = fileIcon{"python", "", "#ffbc03"}
	icoRust     = fileIcon{"rust", "", "#dea584"}
	icoHTML     = fileIcon{"html", "", "#e44d26"}
	icoCSS      = fileIcon{"css", "", "#42a5f5"}
	icoText     = fileIcon{"text", "", "#a0a0a0"}
	icoPDF      = fileIcon{"pdf", "", "#e45649"}
	icoArchive  = fileIcon{"archive", "", "#c9a26d"}
	icoLicense  = fileIcon{"license", "", "#d0bf41"}
)

// iconByName matches whole file names (lower-case) and wins over extensions.
var iconByName = map[string]fileIcon{
	"go.mod":         IcoGo,
	"go.sum":         IcoGo,
	"flake.lock":     icoNix,
	"license":        icoLicense,
	"makefile":       icoConfig,
	"dockerfile":     icoConfig,
	".gitignore":     icoGit,
	".gitattributes": icoGit,
	".gitmodules":    icoGit,
}

// iconByExt is keyed by lower-case extension without the dot.
var iconByExt = map[string]fileIcon{
	"go": IcoGo, "nix": icoNix, "md": icoMarkdown, "markdown": icoMarkdown,
	"png": icoImage, "jpg": icoImage, "jpeg": icoImage, "gif": icoImage,
	"webp": icoImage, "bmp": icoImage, "svg": icoImage, "ico": icoImage,
	"json": icoJSON, "toml": icoConfig, "yaml": icoConfig, "yml": icoConfig,
	"sh": icoShell, "bash": icoShell, "zsh": icoShell,
	"lock": icoLock, "js": icoJS, "mjs": icoJS, "ts": icoTS, "tsx": icoTS,
	"py": icoPython, "rs": icoRust, "html": icoHTML, "css": icoCSS,
	"txt": icoText, "pdf": icoPDF,
	"zip": icoArchive, "tar": icoArchive, "gz": icoArchive,
}

// iconFor picks the icon for a tree node, with the active theme's override
// applied.
func IconFor(n *Node) fileIcon {
	return themedIcon(baseIconFor(n))
}

// themedIcon applies the active theme's override for ic, if it has one.
func themedIcon(ic fileIcon) fileIcon {
	o, ok := CurTheme.Icons[ic.kind]
	if !ok {
		return ic
	}
	if o.Glyph != "" {
		ic.Glyph = o.Glyph
	}
	if o.Color != "" {
		ic.Color = o.Color
	}
	return ic
}

// baseIconFor picks the theme-independent icon for a tree node.
func baseIconFor(n *Node) fileIcon {
	if n.isDir {
		if n.expanded {
			return iconFolderOpen
		}
		return iconFolder
	}
	name := strings.ToLower(n.Name)
	if ic, ok := iconByName[name]; ok {
		return ic
	}
	if ic, ok := iconByExt[strings.TrimPrefix(filepath.Ext(name), ".")]; ok {
		return ic
	}
	return iconDefault
}
